package com.ankitsriv89.whatsapp.dto;

public record KafkaReceiptEvent(Long messageId, Long deviceId, String state) {}
