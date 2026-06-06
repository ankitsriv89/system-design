package com.ankitsriv89.instagram.store;

import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Component;

/**
 * Hot like-counters in Redis (key {@code likes:{postId}}). Postgres holds the
 * durable engagement rows (source of truth); Redis serves the fast read/aggregate
 * path. INCR/DECR are atomic so concurrent likes are safe.
 */
@Component
public class CounterStore {

    private static final String LIKES_PREFIX = "likes:";

    private final StringRedisTemplate redis;

    public CounterStore(StringRedisTemplate redis) {
        this.redis = redis;
    }

    private String likesKey(long postId) {
        return LIKES_PREFIX + postId;
    }

    public long incrementLikes(long postId) {
        Long v = redis.opsForValue().increment(likesKey(postId));
        return v == null ? 0 : v;
    }

    public long decrementLikes(long postId) {
        Long v = redis.opsForValue().decrement(likesKey(postId));
        if (v != null && v < 0) {
            redis.opsForValue().set(likesKey(postId), "0");
            return 0;
        }
        return v == null ? 0 : v;
    }

    public long likeCount(long postId) {
        String v = redis.opsForValue().get(likesKey(postId));
        return v == null ? 0 : Long.parseLong(v);
    }

    /** Seed the counter from the durable source of truth (rebuild path). */
    public void setLikes(long postId, long count) {
        redis.opsForValue().set(likesKey(postId), Long.toString(count));
    }
}
