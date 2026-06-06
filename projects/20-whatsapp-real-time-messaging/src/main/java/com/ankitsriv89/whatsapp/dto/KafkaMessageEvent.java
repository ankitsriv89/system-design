package com.ankitsriv89.whatsapp.dto;

import java.time.Instant;

public record KafkaMessageEvent(
        Long messageId,
        String chatId,
        Long senderId,
        String ciphertext,
        Instant createdAt,
        // Recipient user IDs that need to receive this message via WebSocket or offline backlog.
        java.util.List<Long> recipientUserIds
) {}
