package com.ankitsriv89.whatsapp.dto;

import com.ankitsriv89.whatsapp.domain.Message;
import java.time.Instant;
import java.util.Base64;

public record MessageResponse(Long id, String chatId, Long senderId, String ciphertext, Instant createdAt) {
    public static MessageResponse from(Message m) {
        return new MessageResponse(
                m.getId(),
                m.getChatId(),
                m.getSender().getId(),
                Base64.getEncoder().encodeToString(m.getCiphertext()),
                m.getCreatedAt()
        );
    }
}
