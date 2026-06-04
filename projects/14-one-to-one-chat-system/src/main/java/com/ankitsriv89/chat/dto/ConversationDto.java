package com.ankitsriv89.chat.dto;

import com.ankitsriv89.chat.domain.Conversation;
import java.time.Instant;

public record ConversationDto(Long id, String userA, String userB, Instant createdAt, long lastSeq) {
    public static ConversationDto from(Conversation c) {
        return new ConversationDto(c.getId(), c.getUserA(), c.getUserB(), c.getCreatedAt(), c.getLastSeq());
    }
}
