package com.ankitsriv89.chat.dto;

public record PresenceDto(String userId, boolean online, Long lastSeenEpochMs) {}
