package com.ankitsriv89.twitter.store;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.ZSetOperations;
import org.springframework.stereotype.Component;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Materialized home timelines, backed by one Redis sorted set per user:
 * key {@code timeline:{userId}}, member = tweetId (string), score = ranking score.
 *
 * <p>Fanout-on-write pushes tweet IDs into each follower's ZSET (ZADD). Reads use
 * ZREVRANGE to pull the top-N highest-scored tweet IDs. Each timeline is trimmed
 * to a bounded size so memory stays O(users × maxItems) regardless of how
 * prolific the authors are.
 */
@Component
public class TimelineStore {

    private static final String KEY_PREFIX = "timeline:";

    private final StringRedisTemplate redis;
    private final int maxItems;

    public TimelineStore(StringRedisTemplate redis,
                         @Value("${twitter.timeline.max-cached-items}") int maxItems) {
        this.redis = redis;
        this.maxItems = maxItems;
    }

    private String key(String userId) {
        return KEY_PREFIX + userId;
    }

    /**
     * Bulk-insert one tweet into many follower timelines. Per-key ZADDs keep the
     * tutorial clear and correct; at scale this is where you would pipeline or
     * shard the writes.
     */
    public void pushMany(List<String> userIds, long tweetId, double score) {
        String member = Long.toString(tweetId);
        for (String userId : userIds) {
            String k = key(userId);
            redis.opsForZSet().add(k, member, score);
            trim(k);
        }
    }

    private void trim(String k) {
        // Keep only the top {maxItems} by score: remove everything ranked below.
        Long size = redis.opsForZSet().zCard(k);
        if (size != null && size > maxItems) {
            redis.opsForZSet().removeRange(k, 0, size - maxItems - 1);
        }
    }

    /** Top-N (tweetId -> score), highest score first. */
    public Map<Long, Double> topN(String userId, int n) {
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

    public long size(String userId) {
        Long c = redis.opsForZSet().zCard(key(userId));
        return c == null ? 0 : c;
    }

    /** Replace a user's entire timeline (used by backfill). */
    public void replace(String userId, Map<Long, Double> scored) {
        String k = key(userId);
        redis.delete(k);
        for (var e : scored.entrySet()) {
            redis.opsForZSet().add(k, Long.toString(e.getKey()), e.getValue());
        }
        trim(k);
    }
}
