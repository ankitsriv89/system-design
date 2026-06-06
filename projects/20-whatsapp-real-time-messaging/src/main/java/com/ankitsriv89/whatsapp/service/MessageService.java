package com.ankitsriv89.whatsapp.service;

import com.ankitsriv89.whatsapp.domain.*;
import com.ankitsriv89.whatsapp.dto.*;
import com.ankitsriv89.whatsapp.repository.*;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.domain.PageRequest;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.Instant;
import java.util.Base64;
import java.util.List;

@Service
public class MessageService {

    private final MessageRepository messages;
    private final UserRepository users;
    private final DeviceRepository devices;
    private final ReceiptRepository receipts;
    private final GroupMemberRepository groupMembers;
    private final KafkaTemplate<String, String> kafkaTemplate;
    private final ObjectMapper mapper;
    private final String messagesTopic;
    private final int syncPageSize;

    public MessageService(
            MessageRepository messages,
            UserRepository users,
            DeviceRepository devices,
            ReceiptRepository receipts,
            GroupMemberRepository groupMembers,
            KafkaTemplate<String, String> kafkaTemplate,
            ObjectMapper mapper,
            @Value("${whatsapp.kafka.topic.messages}") String messagesTopic,
            @Value("${whatsapp.delivery.sync-page-size:200}") int syncPageSize) {
        this.messages = messages;
        this.users = users;
        this.devices = devices;
        this.receipts = receipts;
        this.groupMembers = groupMembers;
        this.kafkaTemplate = kafkaTemplate;
        this.mapper = mapper;
        this.messagesTopic = messagesTopic;
        this.syncPageSize = syncPageSize;
    }

    @Transactional
    public MessageResponse send(String senderUsername, SendMessageRequest req) {
        // Verify the sender is a participant before writing anything.
        assertParticipant(senderUsername, req.chatId());

        AppUser sender = users.findByUsername(senderUsername)
                .orElseThrow(() -> new IllegalStateException("Sender not found"));

        byte[] ciphertext = Base64.getDecoder().decode(req.ciphertext());
        Message msg = messages.save(new Message(req.chatId(), sender, ciphertext));

        List<Long> recipientUserIds = resolveRecipients(req.chatId(), sender.getId());

        for (Long userId : recipientUserIds) {
            for (Device d : devices.findByUserId(userId)) {
                receipts.save(new Receipt(msg, d));
            }
        }

        KafkaMessageEvent event = new KafkaMessageEvent(
                msg.getId(), msg.getChatId(), sender.getId(),
                req.ciphertext(), msg.getCreatedAt(), recipientUserIds);
        try {
            kafkaTemplate.send(messagesTopic, req.chatId(), mapper.writeValueAsString(event));
        } catch (Exception e) {
            throw new RuntimeException("Failed to publish message event", e);
        }

        return MessageResponse.from(msg);
    }

    private List<Long> resolveRecipients(String chatId, Long senderId) {
        if (chatId.startsWith("group:")) {
            long groupId = Long.parseLong(chatId.substring(6));
            return groupMembers.findByGroupId(groupId).stream()
                    .map(m -> m.getUser().getId())
                    .filter(uid -> !uid.equals(senderId))
                    .toList();
        }
        if (chatId.startsWith("dm:")) {
            String[] parts = chatId.split(":");
            return java.util.Arrays.stream(parts, 1, parts.length)
                    .map(Long::parseLong)
                    .filter(uid -> !uid.equals(senderId))
                    .toList();
        }
        return List.of();
    }

    public List<MessageResponse> sync(String callerUsername, String chatId, Instant since) {
        assertParticipant(callerUsername, chatId);
        var page = PageRequest.of(0, syncPageSize);
        List<Message> msgs = (since == null)
                ? messages.findByChatIdOrderByCreatedAtAsc(chatId, page)
                : messages.findByChatIdAndCreatedAtAfterOrderByCreatedAtAsc(chatId, since, page);
        return msgs.stream().map(MessageResponse::from).toList();
    }

    /** Throws 403 if the caller is not a participant of the chat. */
    private void assertParticipant(String callerUsername, String chatId) {
        AppUser caller = users.findByUsername(callerUsername)
                .orElseThrow(() -> new org.springframework.security.access.AccessDeniedException("Unknown user"));
        Long callerId = caller.getId();

        if (chatId.startsWith("dm:")) {
            boolean member = java.util.Arrays.stream(chatId.split(":"), 1, chatId.split(":").length)
                    .map(Long::parseLong)
                    .anyMatch(id -> id.equals(callerId));
            if (!member) throw new org.springframework.security.access.AccessDeniedException("Not a participant");
            return;
        }
        if (chatId.startsWith("group:")) {
            long groupId = Long.parseLong(chatId.substring(6));
            if (!groupMembers.existsByGroupIdAndUserId(groupId, callerId)) {
                throw new org.springframework.security.access.AccessDeniedException("Not a group member");
            }
            return;
        }
        throw new org.springframework.security.access.AccessDeniedException("Unknown chat type");
    }
}
