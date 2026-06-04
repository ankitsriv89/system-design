package com.ankitsriv89.chat.dto;

import com.ankitsriv89.chat.domain.Message;
import java.time.Instant;

public record MessageDto(
    Long id,
    Long conversationId,
    String senderId,
    String body,
    long seq,
    String status,
    Instant createdAt
) {
    public static MessageDto from(Message m) {
        return new MessageDto(
            m.getId(), m.getConversationId(), m.getSenderId(),
            m.getBody(), m.getSeq(), m.getStatus().name(), m.getCreatedAt()
        );
    }
}
