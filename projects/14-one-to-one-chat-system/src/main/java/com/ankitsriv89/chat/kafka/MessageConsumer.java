package com.ankitsriv89.chat.kafka;

import com.ankitsriv89.chat.controller.WebSocketHub;
import com.ankitsriv89.chat.dto.KafkaMessageEvent;
import com.ankitsriv89.chat.dto.ReceiptEvent;
import com.ankitsriv89.chat.service.MessageService;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.stereotype.Component;

@Component
public class MessageConsumer {

    private static final Logger log = LoggerFactory.getLogger(MessageConsumer.class);

    private final WebSocketHub hub;
    private final MessageService messageService;

    public MessageConsumer(WebSocketHub hub, MessageService messageService) {
        this.hub = hub;
        this.messageService = messageService;
    }

    @KafkaListener(topics = "${chat.kafka.topic.messages}", groupId = "chat-service",
                   containerFactory = "messageKafkaListenerContainerFactory")
    public void onMessage(KafkaMessageEvent event) {
        log.debug("Kafka message event for recipient={}", event.recipientId());
        boolean delivered = hub.deliver(event.recipientId(), event.message());
        if (delivered) {
            messageService.markDelivered(event.message().id());
        }
        // If not delivered (user offline), message already persisted — pulled on reconnect.
    }

    @KafkaListener(topics = "${chat.kafka.topic.receipts}", groupId = "chat-service",
                   containerFactory = "receiptKafkaListenerContainerFactory")
    public void onReceipt(ReceiptEvent event) {
        log.debug("Kafka receipt event messageId={} status={}", event.messageId(), event.status());
        if ("READ".equals(event.status())) {
            messageService.markRead(event.messageId());
        }
        // Forward receipt to sender over WebSocket
        hub.deliverReceipt(event);
    }
}
