package com.ankitsriv89.chat.store;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Component;

import java.time.Duration;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

// Tracks online presence using Redis keys with TTL.
// A user is "online" as long as their heartbeat key exists; the WebSocket
// session renews it on every received frame, and it expires automatically
// when the connection drops.
@Component
public class PresenceStore {

    private static final String KEY_PREFIX = "presence:";

    private final StringRedisTemplate redis;
    private final Duration ttl;

    public PresenceStore(StringRedisTemplate redis,
                         @Value("${chat.presence.ttl-seconds:30}") long ttlSeconds) {
        this.redis = redis;
        this.ttl = Duration.ofSeconds(ttlSeconds);
    }

    public void heartbeat(String userId) {
        redis.opsForValue().set(KEY_PREFIX + userId, String.valueOf(System.currentTimeMillis()), ttl);
    }

    public void offline(String userId) {
        redis.delete(KEY_PREFIX + userId);
    }

    public boolean isOnline(String userId) {
        return Boolean.TRUE.equals(redis.hasKey(KEY_PREFIX + userId));
    }

    public Map<String, Boolean> bulkStatus(List<String> userIds) {
        return userIds.stream().collect(Collectors.toMap(
            id -> id,
            id -> Boolean.TRUE.equals(redis.hasKey(KEY_PREFIX + id))
        ));
    }

    public Long lastSeenEpochMs(String userId) {
        String val = redis.opsForValue().get(KEY_PREFIX + userId);
        return val != null ? Long.parseLong(val) : null;
    }
}
