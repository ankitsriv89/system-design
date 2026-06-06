package com.ankitsriv89.whatsapp.dto;

// Wire format for all WebSocket frames (both inbound commands and outbound events).
public record WsEnvelope(String type, Object payload) {}
