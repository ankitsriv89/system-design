package com.ankitsriv89.chat.controller;

import com.ankitsriv89.chat.dto.MessageDto;
import com.ankitsriv89.chat.dto.PresenceDto;
import com.ankitsriv89.chat.dto.ReceiptEvent;
import com.ankitsriv89.chat.dto.WsEnvelope;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.messaging.simp.SimpMessagingTemplate;
import org.springframework.stereotype.Component;

// Routes envelopes to connected WebSocket clients via STOMP user destinations.
// Each authenticated user is subscribed to /user/queue/inbox.
@Component
public class WebSocketHub {

    private static final Logger log = LoggerFactory.getLogger(WebSocketHub.class);
    private static final String INBOX = "/queue/inbox";

    private final SimpMessagingTemplate broker;

    public WebSocketHub(SimpMessagingTemplate broker) {
        this.broker = broker;
    }

    // Returns true if the user has at least one active STOMP session.
    // SimpMessagingTemplate silently drops the message if the user has no session,
    // so "delivery" here means "attempted push to an active session".
    public boolean deliver(String userId, MessageDto message) {
        try {
            broker.convertAndSendToUser(userId, INBOX, WsEnvelope.message(message));
            log.debug("Delivered message id={} to user={}", message.id(), userId);
            return true;
        } catch (Exception e) {
            log.warn("Failed to deliver message id={} to user={}: {}", message.id(), userId, e.getMessage());
            return false;
        }
    }

    public void deliverReceipt(ReceiptEvent receipt) {
        broker.convertAndSendToUser(receipt.userId(), INBOX, WsEnvelope.receipt(receipt));
    }

    public void deliverPresence(String userId, PresenceDto presence) {
        broker.convertAndSendToUser(userId, INBOX, WsEnvelope.presence(presence));
    }
}
