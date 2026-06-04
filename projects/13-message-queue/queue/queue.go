// Package queue implements the core message-queue domain: topics, messages,
// consumer groups, visibility timeouts, and dead-letter queues.
package queue

import (
	"errors"
	"time"
)

// DefaultVisibilityTimeout is how long a polled message stays invisible before
// it becomes re-deliverable if the consumer never acks it.
const DefaultVisibilityTimeout = 30 * time.Second

// MaxDeliveryAttempts is the number of times a message is delivered before it
// is moved to the dead-letter queue.
const MaxDeliveryAttempts = 5

// ErrTopicNotFound is returned when the requested topic does not exist.
var ErrTopicNotFound = errors.New("topic not found")

// ErrMessageNotFound is returned when a message ID does not exist.
var ErrMessageNotFound = errors.New("message not found")

// ErrTopicExists is returned when trying to create a topic that already exists.
var ErrTopicExists = errors.New("topic already exists")

// Topic is the logical channel messages are published to and consumed from.
type Topic struct {
	Name            string
	Partitions      int
	RetentionPeriod time.Duration
	CreatedAt       time.Time
}

// Message is a single unit of work stored in a partition of a topic.
type Message struct {
	ID               string
	Topic            string
	Partition        int
	Offset           int64
	Key              string
	Payload          []byte
	PublishedAt      time.Time
	VisibleAt        time.Time // time after which the message is re-deliverable
	DeliveryAttempts int
	AckedAt          *time.Time
	DeadLettered     bool
	ConsumerGroup    string
}

// ConsumerOffset tracks how far a consumer group has read into a partition.
type ConsumerOffset struct {
	Group     string
	Topic     string
	Partition int
	Offset    int64
	UpdatedAt time.Time
}

// PublishRequest is the input for publishing a message.
type PublishRequest struct {
	Topic   string
	Key     string
	Payload []byte
}

// PollRequest is the input for polling messages.
type PollRequest struct {
	Topic             string
	ConsumerGroup     string
	Partition         int // -1 means any partition
	MaxMessages       int
	VisibilityTimeout time.Duration
}

// AckRequest is the input for acknowledging a message.
type AckRequest struct {
	MessageID     string
	ConsumerGroup string
}

// PartitionFor returns the partition index for the given key using a simple
// modulo hash. An empty key falls back to round-robin (caller must supply
// a monotonic counter as tiebreaker via the counter param).
func PartitionFor(key string, partitions int, counter int64) int {
	if partitions <= 1 {
		return 0
	}
	if key == "" {
		return int(counter % int64(partitions))
	}
	// FNV-1a hash for deterministic key-based routing.
	var h uint64 = 14695981039346656037
	for i := 0; i < len(key); i++ {
		h ^= uint64(key[i])
		h *= 1099511628211
	}
	return int(h % uint64(partitions))
}
