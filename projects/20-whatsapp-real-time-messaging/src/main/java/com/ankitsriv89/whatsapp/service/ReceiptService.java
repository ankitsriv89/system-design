package com.ankitsriv89.whatsapp.service;

import com.ankitsriv89.whatsapp.domain.*;
import com.ankitsriv89.whatsapp.dto.KafkaReceiptEvent;
import com.ankitsriv89.whatsapp.repository.DeviceRepository;
import com.ankitsriv89.whatsapp.repository.GroupMemberRepository;
import com.ankitsriv89.whatsapp.repository.MessageRepository;
import com.ankitsriv89.whatsapp.repository.ReceiptRepository;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.security.access.AccessDeniedException;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.Arrays;
import java.util.List;

@Service
public class ReceiptService {

    private final ReceiptRepository receipts;
    private final DeviceRepository devices;
    private final MessageRepository messages;
    private final GroupMemberRepository groupMembers;
    private final KafkaTemplate<String, String> kafkaTemplate;
    private final ObjectMapper mapper;
    private final String receiptsTopic;

    public ReceiptService(
            ReceiptRepository receipts,
            DeviceRepository devices,
            MessageRepository messages,
            GroupMemberRepository groupMembers,
            KafkaTemplate<String, String> kafkaTemplate,
            ObjectMapper mapper,
            @Value("${whatsapp.kafka.topic.receipts}") String receiptsTopic) {
        this.receipts = receipts;
        this.devices = devices;
        this.messages = messages;
        this.groupMembers = groupMembers;
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
        advanceInternal(messageId, deviceId, ReceiptState.valueOf(stateStr.toUpperCase()));
    }

    public List<Receipt> pendingForDevice(Long deviceId) {
        return receipts.findPendingForDevice(deviceId);
    }

    /** Called from WebSocket handler where device ownership was already verified at connect time. */
    @Transactional
    public void advanceFromDevice(Long messageId, Long deviceId, String stateStr) {
        advanceInternal(messageId, deviceId, ReceiptState.valueOf(stateStr.toUpperCase()));
    }

    /** Internal server-side delivery mark — skips ownership check (called by Kafka router). */
    @Transactional
    public void markDelivered(Long messageId, Long deviceId) {
        advanceInternal(messageId, deviceId, ReceiptState.DELIVERED);
    }

    private void advanceInternal(Long messageId, Long deviceId, ReceiptState next) {
        receipts.findById(new ReceiptId(messageId, deviceId)).ifPresent(r -> {
            ReceiptState current = ReceiptState.valueOf(r.getState());
            if (!current.canAdvanceTo(next)) return;
            r.advance(next);
            receipts.save(r);
            publish(messageId, deviceId, next, r.getMessage().getChatId());
        });
    }

    private void publish(Long messageId, Long deviceId, ReceiptState state, String chatId) {
        List<Long> participants = resolveParticipants(chatId);
        try {
            kafkaTemplate.send(receiptsTopic, String.valueOf(messageId),
                    mapper.writeValueAsString(
                            new KafkaReceiptEvent(messageId, deviceId, state.name(), chatId, participants)));
        } catch (Exception e) {
            throw new RuntimeException("Failed to publish receipt event", e);
        }
    }

    /** Resolves all user IDs that are participants in a chat (for scoped receipt push). */
    private List<Long> resolveParticipants(String chatId) {
        if (chatId.startsWith("dm:")) {
            return Arrays.stream(chatId.split(":"), 1, chatId.split(":").length)
                    .map(Long::parseLong)
                    .toList();
        }
        if (chatId.startsWith("group:")) {
            long groupId = Long.parseLong(chatId.substring(6));
            return groupMembers.findByGroupId(groupId).stream()
                    .map(m -> m.getUser().getId())
                    .toList();
        }
        return List.of();
    }
}
