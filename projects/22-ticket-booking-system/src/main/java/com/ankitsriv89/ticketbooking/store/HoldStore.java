package com.ankitsriv89.ticketbooking.store;

import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Component;

import java.time.Duration;

/**
 * Redis-backed hold TTL tracker. Mirrors hold expiry so the scheduler can sweep
 * cheaply without a full DB scan on every tick.
 */
@Component
public class HoldStore {

    private static final String KEY_PREFIX = "hold:";
    private static final String SEAT_HOLD_PREFIX = "seat_hold:";

    private final StringRedisTemplate redis;

    public HoldStore(StringRedisTemplate redis) {
        this.redis = redis;
    }

    public void registerHold(String holdId, String seatId, Duration ttl) {
        redis.opsForValue().set(KEY_PREFIX + holdId, seatId, ttl);
        redis.opsForValue().set(SEAT_HOLD_PREFIX + seatId, holdId, ttl);
    }

    public boolean isSeatHeld(String seatId) {
        return Boolean.TRUE.equals(redis.hasKey(SEAT_HOLD_PREFIX + seatId));
    }

    public void releaseHold(String holdId, String seatId) {
        redis.delete(KEY_PREFIX + holdId);
        redis.delete(SEAT_HOLD_PREFIX + seatId);
    }

    public String getHoldIdForSeat(String seatId) {
        return redis.opsForValue().get(SEAT_HOLD_PREFIX + seatId);
    }
}
