package com.ankitsriv89.chat.controller;

import com.ankitsriv89.chat.dto.ReceiptEvent;
import com.ankitsriv89.chat.dto.SendMessageRequest;
import com.ankitsriv89.chat.dto.WsEnvelope;
import com.ankitsriv89.chat.kafka.MessageProducer;
import com.ankitsriv89.chat.service.MessageService;
import com.ankitsriv89.chat.service.PresenceService;
import org.springframework.messaging.handler.annotation.MessageMapping;
import org.springframework.messaging.handler.annotation.Payload;
import org.springframework.stereotype.Controller;

import java.security.Principal;

// Handles inbound STOMP frames from connected clients.
// Clients subscribe to /user/queue/inbox to receive messages and receipts.
@Controller
public class ChatController {

    private final MessageService msgService;
    private final PresenceService presenceService;
    private final MessageProducer producer;

    public ChatController(MessageService msgService, PresenceService presenceService,
                          MessageProducer producer) {
        this.msgService = msgService;
        this.presenceService = presenceService;
        this.producer = producer;
    }

    // Client sends: SEND /app/chat.send  {"recipientId":"bob","body":"hello"}
    @MessageMapping("/chat.send")
    public void handleSend(@Payload SendMessageRequest req, Principal principal) {
        presenceService.heartbeat(principal.getName());
        msgService.send(principal.getName(), req.recipientId(), req.body());
    }

    // Client sends: SEND /app/chat.read  {"messageId":123,"status":"READ"}
    @MessageMapping("/chat.receipt")
    public void handleReceipt(@Payload ReceiptEvent receipt, Principal principal) {
        presenceService.heartbeat(principal.getName());
        producer.publishReceipt(receipt);
    }

    // Client sends: SEND /app/chat.heartbeat  (any payload)
    @MessageMapping("/chat.heartbeat")
    public void handleHeartbeat(Principal principal) {
        presenceService.heartbeat(principal.getName());
    }
}
