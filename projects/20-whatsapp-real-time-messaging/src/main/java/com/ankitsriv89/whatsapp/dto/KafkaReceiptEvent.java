package com.ankitsriv89.whatsapp.dto;

import java.util.List;

public record KafkaReceiptEvent(
        Long messageId,
        Long deviceId,
        String state,
        // chatId and participant user IDs carried so the router can scope WS pushes.
        String chatId,
        List<Long> participantUserIds
) {}
