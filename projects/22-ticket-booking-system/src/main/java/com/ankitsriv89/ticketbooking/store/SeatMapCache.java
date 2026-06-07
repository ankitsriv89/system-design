package com.ankitsriv89.ticketbooking.store;

import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Component;

import java.time.Duration;

/**
 * Caches serialized seat-map JSON for an event. Bypassed (or invalidated) on
 * any seat status change so reads are never stale.
 */
@Component
public class SeatMapCache {

    private static final String KEY_PREFIX = "seatmap:";
    private static final Duration TTL = Duration.ofSeconds(30);

    private final StringRedisTemplate redis;

    public SeatMapCache(StringRedisTemplate redis) {
        this.redis = redis;
    }

    public void put(String eventId, String json) {
        redis.opsForValue().set(KEY_PREFIX + eventId, json, TTL);
    }

    public String get(String eventId) {
        return redis.opsForValue().get(KEY_PREFIX + eventId);
    }

    public void evict(String eventId) {
        redis.delete(KEY_PREFIX + eventId);
    }
}
