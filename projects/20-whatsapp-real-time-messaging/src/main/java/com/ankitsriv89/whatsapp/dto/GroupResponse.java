package com.ankitsriv89.whatsapp.dto;

import com.ankitsriv89.whatsapp.domain.ChatGroup;

public record GroupResponse(Long id, String name, String chatId, Long ownerId) {
    public static GroupResponse from(ChatGroup g) {
        return new GroupResponse(g.getId(), g.getName(), g.chatId(), g.getOwner().getId());
    }
}
