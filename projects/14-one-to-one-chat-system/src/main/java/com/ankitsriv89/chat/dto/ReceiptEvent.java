package com.ankitsriv89.chat.dto;

// Kafka-serialisable delivery/read receipt event.
public record ReceiptEvent(Long messageId, String userId, String status) {}
