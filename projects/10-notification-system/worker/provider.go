// Package worker implements the async dispatch pipeline with provider mocks, retry, and DLQ.
package worker

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go.uber.org/zap"

	"github.com/ankitsriv89/10-notification-system/notification"
)

// Provider is the interface each channel backend implements.
type Provider interface {
	Name() string
	Channel() notification.Channel
	// Send returns nil on success, error on failure.
	Send(ctx context.Context, n *notification.Notification) error
}

// ── Mock providers ────────────────────────────────────────────────────────────

// MockEmailProvider simulates an SMTP relay with configurable failure rate.
type MockEmailProvider struct {
	FailureRate float64 // 0.0–1.0
	LatencyMs   int     // simulated send latency
	log         *zap.Logger
}

func NewMockEmailProvider(failureRate float64, latencyMs int, log *zap.Logger) *MockEmailProvider {
	return &MockEmailProvider{FailureRate: failureRate, LatencyMs: latencyMs, log: log}
}

func (p *MockEmailProvider) Name() string                   { return "mock-email" }
func (p *MockEmailProvider) Channel() notification.Channel  { return notification.ChannelEmail }

func (p *MockEmailProvider) Send(ctx context.Context, n *notification.Notification) error {
	time.Sleep(time.Duration(p.LatencyMs) * time.Millisecond)
	if rand.Float64() < p.FailureRate {
		return fmt.Errorf("mock-email: transient SMTP error")
	}
	p.log.Info("mock-email sent",
		zap.String("notification_id", n.ID),
		zap.String("user_id", n.UserID),
		zap.String("subject", n.Subject),
	)
	return nil
}

// MockSMSProvider simulates an SMS gateway.
type MockSMSProvider struct {
	FailureRate float64
	LatencyMs   int
	log         *zap.Logger
}

func NewMockSMSProvider(failureRate float64, latencyMs int, log *zap.Logger) *MockSMSProvider {
	return &MockSMSProvider{FailureRate: failureRate, LatencyMs: latencyMs, log: log}
}

func (p *MockSMSProvider) Name() string                   { return "mock-sms" }
func (p *MockSMSProvider) Channel() notification.Channel  { return notification.ChannelSMS }

func (p *MockSMSProvider) Send(ctx context.Context, n *notification.Notification) error {
	time.Sleep(time.Duration(p.LatencyMs) * time.Millisecond)
	if rand.Float64() < p.FailureRate {
		return fmt.Errorf("mock-sms: gateway timeout")
	}
	p.log.Info("mock-sms sent",
		zap.String("notification_id", n.ID),
		zap.String("user_id", n.UserID),
		zap.String("body", n.Body[:min(len(n.Body), 40)]),
	)
	return nil
}

// MockPushProvider simulates a mobile push service (APNs/FCM).
type MockPushProvider struct {
	FailureRate float64
	LatencyMs   int
	log         *zap.Logger
}

func NewMockPushProvider(failureRate float64, latencyMs int, log *zap.Logger) *MockPushProvider {
	return &MockPushProvider{FailureRate: failureRate, LatencyMs: latencyMs, log: log}
}

func (p *MockPushProvider) Name() string                   { return "mock-push" }
func (p *MockPushProvider) Channel() notification.Channel  { return notification.ChannelPush }

func (p *MockPushProvider) Send(ctx context.Context, n *notification.Notification) error {
	time.Sleep(time.Duration(p.LatencyMs) * time.Millisecond)
	if rand.Float64() < p.FailureRate {
		return fmt.Errorf("mock-push: device token expired")
	}
	p.log.Info("mock-push sent",
		zap.String("notification_id", n.ID),
		zap.String("user_id", n.UserID),
	)
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
