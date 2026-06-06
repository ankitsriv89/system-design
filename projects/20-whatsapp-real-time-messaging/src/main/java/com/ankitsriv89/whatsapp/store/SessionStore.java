package com.ankitsriv89.whatsapp.store;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Component;
import org.springframework.web.socket.WebSocketSession;

import java.time.Duration;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

/**
 * Tracks live WebSocket sessions in-process (for single-node) and publishes
 * device→node route hints in Redis so future multi-node routing can look up
 * which node holds a given device's connection.
 *
 * Redis key: ws:route:{deviceId} → "node-id" (TTL refreshed on heartbeat)
 */
@Component
public class SessionStore {

    private static final String NODE_ID = java.util.UUID.randomUUID().toString().substring(0, 8);
    private static final String KEY_PREFIX = "ws:route:";

    private final StringRedisTemplate redis;
    private final Duration routeTtl;

    // In-process WebSocket session map: deviceId → WebSocketSession
    private final Map<Long, WebSocketSession> localSessions = new ConcurrentHashMap<>();

    public SessionStore(StringRedisTemplate redis,
                        @Value("${whatsapp.session.route-ttl-seconds:90}") long routeTtlSeconds) {
        this.redis = redis;
        this.routeTtl = Duration.ofSeconds(routeTtlSeconds);
    }

    public void register(Long deviceId, WebSocketSession session) {
        localSessions.put(deviceId, session);
        redis.opsForValue().set(KEY_PREFIX + deviceId, NODE_ID, routeTtl);
    }

    public void remove(Long deviceId) {
        localSessions.remove(deviceId);
        redis.delete(KEY_PREFIX + deviceId);
    }

    public WebSocketSession get(Long deviceId) {
        return localSessions.get(deviceId);
    }

    public boolean isOnline(Long deviceId) {
        return localSessions.containsKey(deviceId);
    }

    /** Refresh the Redis TTL (called on heartbeat / incoming frame). */
    public void heartbeat(Long deviceId) {
        redis.expire(KEY_PREFIX + deviceId, routeTtl);
    }

    public Map<Long, WebSocketSession> allSessions() {
        return java.util.Collections.unmodifiableMap(localSessions);
    }
}
