package com.ankitsriv89.groupchat.store;

import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Component;

import java.time.Duration;
import java.util.List;
import java.util.Set;
import java.util.stream.Collectors;

@Component
public class PresenceStore {

    private static final String KEY_PREFIX = "groupchat:presence:";
    private static final Duration TTL = Duration.ofSeconds(30);

    private final StringRedisTemplate redis;

    public PresenceStore(StringRedisTemplate redis) {
        this.redis = redis;
    }

    public void setOnline(String userId) {
        redis.opsForValue().set(KEY_PREFIX + userId, "1", TTL);
    }

    public void setOffline(String userId) {
        redis.delete(KEY_PREFIX + userId);
    }

    public boolean isOnline(String userId) {
        return Boolean.TRUE.equals(redis.hasKey(KEY_PREFIX + userId));
    }

    public Set<String> onlineAmong(List<String> userIds) {
        return userIds.stream()
                .filter(this::isOnline)
                .collect(Collectors.toSet());
    }
}
