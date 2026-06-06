package com.ankitsriv89.whatsapp.store;

import com.ankitsriv89.whatsapp.api.SessionHandler;
import com.ankitsriv89.whatsapp.domain.Device;
import com.ankitsriv89.whatsapp.dto.KafkaMessageEvent;
import com.ankitsriv89.whatsapp.dto.KafkaReceiptEvent;
import com.ankitsriv89.whatsapp.dto.WsEnvelope;
import com.ankitsriv89.whatsapp.repository.DeviceRepository;
import com.ankitsriv89.whatsapp.service.ReceiptService;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.apache.kafka.clients.consumer.ConsumerRecord;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.Map;

/**
 * Kafka consumer that delivers messages and receipt updates to connected devices via WebSocket.
 *
 * Flow for a sent message:
 *   1. MessageService publishes KafkaMessageEvent partitioned by chatId.
 *   2. MessageRouter receives it, looks up each recipient's devices.
 *   3. For each device that is online, pushes a "message" WS frame and marks receipt DELIVERED.
 *   4. Offline devices keep state = SENT; they drain via /v1/messages/sync on reconnect.
 */
@Component
public class MessageRouter {

    private static final Logger log = LoggerFactory.getLogger(MessageRouter.class);

    private final SessionHandler sessionHandler;
    private final SessionStore sessionStore;
    private final DeviceRepository devices;
    private final ReceiptService receiptService;
    private final ObjectMapper mapper;

    public MessageRouter(SessionHandler sessionHandler, SessionStore sessionStore,
                         DeviceRepository devices, ReceiptService receiptService,
                         ObjectMapper mapper) {
        this.sessionHandler = sessionHandler;
        this.sessionStore = sessionStore;
        this.devices = devices;
        this.receiptService = receiptService;
        this.mapper = mapper;
    }

    @KafkaListener(topics = "${whatsapp.kafka.topic.messages}", groupId = "whatsapp-router",
                   containerFactory = "kafkaListenerContainerFactory")
    public void onMessage(ConsumerRecord<String, String> record) {
        KafkaMessageEvent event;
        try {
            event = mapper.readValue(record.value(), KafkaMessageEvent.class);
        } catch (Exception e) {
            log.error("failed to deserialize message event: {}", e.getMessage());
            return;
        }

        log.debug("routing message {} to {} recipients", event.messageId(), event.recipientUserIds().size());

        Map<String, Object> payload = Map.of(
                "id", event.messageId(),
                "chatId", event.chatId(),
                "senderId", event.senderId(),
                "ciphertext", event.ciphertext(),
                "createdAt", event.createdAt().toString()
        );
        WsEnvelope envelope = new WsEnvelope("message", payload);

        for (Long userId : event.recipientUserIds()) {
            List<Device> userDevices = devices.findByUserId(userId);
            for (Device device : userDevices) {
                sessionHandler.push(device.getId(), envelope);
                try {
                    receiptService.markDelivered(event.messageId(), device.getId());
                } catch (Exception e) {
                    log.warn("could not mark delivered for device {}: {}", device.getId(), e.getMessage());
                }
            }
        }
    }

    @KafkaListener(topics = "${whatsapp.kafka.topic.receipts}", groupId = "whatsapp-receipt-fan",
                   containerFactory = "kafkaListenerContainerFactory")
    public void onReceipt(ConsumerRecord<String, String> record) {
        KafkaReceiptEvent event;
        try {
            event = mapper.readValue(record.value(), KafkaReceiptEvent.class);
        } catch (Exception e) {
            log.error("failed to deserialize receipt event: {}", e.getMessage());
            return;
        }

        // Push receipt state back to all connected sessions; clients filter by messageId.
        WsEnvelope envelope = new WsEnvelope("receipt", Map.of(
                "messageId", event.messageId(),
                "deviceId", event.deviceId(),
                "state", event.state()
        ));
        sessionStore.allSessions().keySet().forEach(deviceId ->
                sessionHandler.push(deviceId, envelope));
    }
}
