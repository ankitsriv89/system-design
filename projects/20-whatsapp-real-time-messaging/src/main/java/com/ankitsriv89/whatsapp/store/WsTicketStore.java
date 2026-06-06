package com.ankitsriv89.whatsapp.store;

import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Component;

import java.time.Duration;
import java.util.UUID;

/**
 * Issues short-lived, single-use opaque tickets for WebSocket upgrades.
 * A ticket is redeemed at most once; it replaces the JWT in the query string,
 * keeping the long-lived token out of server access logs and browser history.
 *
 * Redis key: ws:ticket:{uuid} → "{username}:{deviceId}"   TTL 30 s
 */
@Component
public class WsTicketStore {

    private static final String PREFIX = "ws:ticket:";
    private static final Duration TTL = Duration.ofSeconds(30);

    private final StringRedisTemplate redis;

    public WsTicketStore(StringRedisTemplate redis) { this.redis = redis; }

    public String issue(String username, Long deviceId) {
        String ticket = UUID.randomUUID().toString();
        redis.opsForValue().set(PREFIX + ticket, username + ":" + deviceId, TTL);
        return ticket;
    }

    /** Redeems a ticket exactly once. Returns null if the ticket is unknown or expired. */
    public String[] redeem(String ticket) {
        String val = redis.opsForValue().getAndDelete(PREFIX + ticket);
        if (val == null) return null;
        return val.split(":", 2);
    }
}
