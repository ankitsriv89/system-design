package com.ankitsriv89.chat.dto;

// WebSocket message envelope — wraps all inbound and outbound WS payloads.
public record WsEnvelope(String type, Object payload) {
    public static WsEnvelope message(MessageDto msg) { return new WsEnvelope("MESSAGE", msg); }
    public static WsEnvelope receipt(ReceiptEvent r)  { return new WsEnvelope("RECEIPT", r); }
    public static WsEnvelope presence(PresenceDto p)  { return new WsEnvelope("PRESENCE", p); }
    public static WsEnvelope error(String msg)        { return new WsEnvelope("ERROR", msg); }
}
