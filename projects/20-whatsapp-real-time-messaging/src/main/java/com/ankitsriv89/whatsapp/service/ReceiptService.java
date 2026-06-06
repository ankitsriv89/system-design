package com.ankitsriv89.whatsapp.service;

import com.ankitsriv89.whatsapp.domain.*;
import com.ankitsriv89.whatsapp.dto.KafkaReceiptEvent;
import com.ankitsriv89.whatsapp.repository.DeviceRepository;
import com.ankitsriv89.whatsapp.repository.ReceiptRepository;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.security.access.AccessDeniedException;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;

@Service
public class ReceiptService {

    private final ReceiptRepository receipts;
    private final DeviceRepository devices;
    private final KafkaTemplate<String, String> kafkaTemplate;
    private final ObjectMapper mapper;
    private final String receiptsTopic;

    public ReceiptService(
            ReceiptRepository receipts,
            DeviceRepository devices,
            KafkaTemplate<String, String> kafkaTemplate,
            ObjectMapper mapper,
            @Value("${whatsapp.kafka.topic.receipts}") String receiptsTopic) {
        this.receipts = receipts;
        this.devices = devices;
        this.kafkaTemplate = kafkaTemplate;
        this.mapper = mapper;
        this.receiptsTopic = receiptsTopic;
    }

    @Transactional
    public void advance(String callerUsername, Long messageId, Long deviceId, String stateStr) {
        Device device = devices.findById(deviceId)
                .orElseThrow(() -> new IllegalArgumentException("Device not found"));
        if (!device.getUser().getUsername().equals(callerUsername)) {
            throw new AccessDeniedException("Device not owned by caller");
        }
        ReceiptState next = ReceiptState.valueOf(stateStr.toUpperCase());
        Receipt r = receipts.findById(new ReceiptId(messageId, deviceId))
                .orElseThrow(() -> new IllegalArgumentException("Receipt not found"));
        ReceiptState current = ReceiptState.valueOf(r.getState());
        if (current.canAdvanceTo(next)) {
            r.advance(next);
            receipts.save(r);
            try {
                kafkaTemplate.send(receiptsTopic, String.valueOf(messageId),
                        mapper.writeValueAsString(new KafkaReceiptEvent(messageId, deviceId, next.name())));
            } catch (Exception e) {
                throw new RuntimeException("Failed to publish receipt event", e);
            }
        }
    }

    public List<Receipt> pendingForDevice(Long deviceId) {
        return receipts.findPendingForDevice(deviceId);
    }

    /** Called from WebSocket handler where device ownership was already verified at connect time. */
    @Transactional
    public void advanceFromDevice(Long messageId, Long deviceId, String stateStr) {
        ReceiptState next = ReceiptState.valueOf(stateStr.toUpperCase());
        receipts.findById(new ReceiptId(messageId, deviceId)).ifPresent(r -> {
            ReceiptState current = ReceiptState.valueOf(r.getState());
            if (current.canAdvanceTo(next)) {
                r.advance(next);
                receipts.save(r);
                try {
                    kafkaTemplate.send(receiptsTopic, String.valueOf(messageId),
                            mapper.writeValueAsString(new KafkaReceiptEvent(messageId, deviceId, next.name())));
                } catch (Exception e) {
                    throw new RuntimeException("Failed to publish receipt event", e);
                }
            }
        });
    }

    /** Internal server-side delivery mark — skips ownership check (called by Kafka router). */
    @Transactional
    public void markDelivered(Long messageId, Long deviceId) {
        ReceiptState next = ReceiptState.DELIVERED;
        receipts.findById(new ReceiptId(messageId, deviceId)).ifPresent(r -> {
            ReceiptState current = ReceiptState.valueOf(r.getState());
            if (current.canAdvanceTo(next)) {
                r.advance(next);
                receipts.save(r);
                try {
                    kafkaTemplate.send(receiptsTopic, String.valueOf(messageId),
                            mapper.writeValueAsString(new KafkaReceiptEvent(messageId, deviceId, next.name())));
                } catch (Exception e) {
                    throw new RuntimeException("Failed to publish receipt event", e);
                }
            }
        });
    }
}
