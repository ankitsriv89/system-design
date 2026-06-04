package com.ankitsriv89.chat.dto;

// Kafka-serialisable event published when a message is persisted.
public record KafkaMessageEvent(MessageDto message, String recipientId) {}
