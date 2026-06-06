package com.ankitsriv89.whatsapp.api;

import com.ankitsriv89.whatsapp.domain.Device;
import com.ankitsriv89.whatsapp.dto.WsEnvelope;
import com.ankitsriv89.whatsapp.repository.DeviceRepository;
import com.ankitsriv89.whatsapp.service.ReceiptService;
import com.ankitsriv89.whatsapp.store.SessionStore;
import com.ankitsriv89.whatsapp.store.WsTicketStore;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Component;
import org.springframework.web.socket.*;
import org.springframework.web.socket.handler.TextWebSocketHandler;

import java.io.IOException;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

/**
 * WebSocket session handler for /ws/v1/session.
 *
 * Connection lifecycle:
 *   1. Client connects with ?token=JWT&deviceId=N query params.
 *   2. Handler validates JWT and registers the device route in Redis + in-process map.
 *   3. On connect, any pending offline receipts for the device are drained.
 *   4. Incoming frames: { "type": "ping" } → pong; { "type": "receipt", "payload": {...} } → advance state.
 *   5. On disconnect, route entry is removed from Redis.
 */
@Component
public class SessionHandler extends TextWebSocketHandler {

    private static final Logger log = LoggerFactory.getLogger(SessionHandler.class);

    private final WsTicketStore ticketStore;
    private final DeviceRepository devices;
    private final SessionStore sessionStore;
    private final ReceiptService receiptService;
    private final ObjectMapper mapper;

    // session-id → deviceId (for cleanup on disconnect)
    private final Map<String, Long> sessionDeviceMap = new ConcurrentHashMap<>();

    public SessionHandler(WsTicketStore ticketStore, DeviceRepository devices,
                          SessionStore sessionStore, ReceiptService receiptService,
                          ObjectMapper mapper) {
        this.ticketStore = ticketStore;
        this.devices = devices;
        this.sessionStore = sessionStore;
        this.receiptService = receiptService;
        this.mapper = mapper;
    }

    @Override
    public void afterConnectionEstablished(WebSocketSession session) throws Exception {
        // Ticket is redeemed once and discarded — the JWT never appears in the URL.
        String ticket = queryParam(session, "ticket");
        if (ticket == null) {
            session.close(CloseStatus.POLICY_VIOLATION);
            return;
        }

        String[] parts = ticketStore.redeem(ticket);
        if (parts == null || parts.length != 2) {
            session.close(CloseStatus.POLICY_VIOLATION);
            return;
        }
        String username = parts[0];
        Long deviceId;
        try {
            deviceId = Long.parseLong(parts[1]);
        } catch (NumberFormatException e) {
            session.close(CloseStatus.POLICY_VIOLATION);
            return;
        }

        Device device = devices.findById(deviceId).orElse(null);
        if (device == null || !device.getUser().getUsername().equals(username)) {
            session.close(CloseStatus.POLICY_VIOLATION);
            return;
        }

        device.touch();
        devices.save(device);
        sessionStore.register(deviceId, session);
        sessionDeviceMap.put(session.getId(), deviceId);

        log.info("device {} connected (user={} session={})", deviceId, username, session.getId());

        // Drain pending offline receipts.
        var pending = receiptService.pendingForDevice(deviceId);
        if (!pending.isEmpty()) {
            send(session, new WsEnvelope("backlog", Map.of(
                    "count", pending.size(),
                    "messageIds", pending.stream().map(r -> r.getMessage().getId()).toList()
            )));
        }

        send(session, new WsEnvelope("connected", Map.of("deviceId", deviceId)));
    }

    @Override
    protected void handleTextMessage(WebSocketSession session, TextMessage message) throws Exception {
        Long deviceId = sessionDeviceMap.get(session.getId());
        if (deviceId == null) return;

        sessionStore.heartbeat(deviceId);

        WsEnvelope env;
        try {
            env = mapper.readValue(message.getPayload(), WsEnvelope.class);
        } catch (Exception e) {
            send(session, new WsEnvelope("error", "invalid envelope"));
            return;
        }

        switch (env.type()) {
            case "ping" -> send(session, new WsEnvelope("pong", null));
            case "receipt" -> handleReceipt(session, deviceId, env);
            default -> send(session, new WsEnvelope("error", "unknown type: " + env.type()));
        }
    }

    private void handleReceipt(WebSocketSession session, Long deviceId, WsEnvelope env) throws IOException {
        try {
            @SuppressWarnings("unchecked")
            Map<String, Object> p = (Map<String, Object>) env.payload();
            Long messageId = ((Number) p.get("messageId")).longValue();
            String newState = (String) p.get("state");
            // Device ownership is already verified at connect time via ticket redemption.
            receiptService.advanceFromDevice(messageId, deviceId, newState);
        } catch (Exception e) {
            log.warn("receipt advance failed: {}", e.getMessage());
            send(session, new WsEnvelope("error", "receipt update failed: " + e.getMessage()));
        }
    }

    @Override
    public void afterConnectionClosed(WebSocketSession session, CloseStatus status) {
        Long deviceId = sessionDeviceMap.remove(session.getId());
        if (deviceId != null) {
            sessionStore.remove(deviceId);
            log.info("device {} disconnected ({})", deviceId, status);
        }
    }

    public void push(Long deviceId, WsEnvelope envelope) {
        WebSocketSession session = sessionStore.get(deviceId);
        if (session != null && session.isOpen()) {
            try {
                send(session, envelope);
            } catch (IOException e) {
                log.warn("push to device {} failed: {}", deviceId, e.getMessage());
            }
        }
    }

    private void send(WebSocketSession session, WsEnvelope envelope) throws IOException {
        session.sendMessage(new TextMessage(mapper.writeValueAsString(envelope)));
    }

    private String queryParam(WebSocketSession session, String param) {
        String query = session.getUri() == null ? null : session.getUri().getQuery();
        if (query == null) return null;
        for (String part : query.split("&")) {
            String[] kv = part.split("=", 2);
            if (kv.length == 2 && kv[0].equals(param)) return kv[1];
        }
        return null;
    }
}
