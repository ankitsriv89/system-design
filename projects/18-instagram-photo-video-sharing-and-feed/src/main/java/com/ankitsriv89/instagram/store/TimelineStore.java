package com.ankitsriv89.instagram.store;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.ZSetOperations;
import org.springframework.stereotype.Component;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Materialized home timelines, one Redis sorted set per user:
 * key {@code feed:{userId}}, member = postId, score = ranking score.
 *
 * <p>Fanout-on-write pushes post IDs into each follower's ZSET; reads pull the
 * top-N via ZREVRANGE. Each timeline is trimmed to a bound so memory is
 * O(users × maxItems) regardless of how prolific authors are.
 */
@Component
public class TimelineStore {

    private final StringRedisTemplate redis;
    private final String keyPrefix;
    private final int maxItems;

    public TimelineStore(StringRedisTemplate redis,
                         @Value("${instagram.feed.redis-key-prefix}") String keyPrefix,
                         @Value("${instagram.feed.max-size}") int maxItems) {
        this.redis = redis;
        this.keyPrefix = keyPrefix;
        this.maxItems = maxItems;
    }

    private String key(Long userId) {
        return keyPrefix + userId;
    }

    /** Insert one post into many follower timelines, trimming each to bound. */
    public void pushMany(List<Long> userIds, long postId, double score) {
        String member = Long.toString(postId);
        for (Long userId : userIds) {
            String k = key(userId);
            redis.opsForZSet().add(k, member, score);
            trim(k);
        }
    }

    private void trim(String k) {
        Long size = redis.opsForZSet().zCard(k);
        if (size != null && size > maxItems) {
            redis.opsForZSet().removeRange(k, 0, size - maxItems - 1);
        }
    }

    /** Top-N (postId -> score), highest score first. */
    public Map<Long, Double> topN(Long userId, int n) {
        Set<ZSetOperations.TypedTuple<String>> tuples =
                redis.opsForZSet().reverseRangeWithScores(key(userId), 0, n - 1);
        Map<Long, Double> out = new LinkedHashMap<>();
        if (tuples != null) {
            for (var t : tuples) {
                if (t.getValue() != null && t.getScore() != null) {
                    out.put(Long.parseLong(t.getValue()), t.getScore());
                }
            }
        }
        return out;
    }

    public long size(Long userId) {
        Long c = redis.opsForZSet().zCard(key(userId));
        return c == null ? 0 : c;
    }
}
