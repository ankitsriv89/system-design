package com.ankitsriv89.newsfeed.store;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.ZSetOperations;
import org.springframework.stereotype.Component;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Materialized home timelines, backed by one Redis sorted set per user:
 * key {@code feed:{userId}}, member = postId (as string), score = ranking score.
 *
 * <p>Fanout-on-write pushes post IDs into each follower's ZSET (ZADD). Reads use
 * ZREVRANGE to pull the top-N highest-scored post IDs. Each timeline is trimmed
 * to a bounded size so memory stays O(users × maxItems) regardless of how
 * prolific the authors are.
 */
@Component
public class TimelineStore {

    private static final String KEY_PREFIX = "feed:";

    private final StringRedisTemplate redis;
    private final int maxItems;

    public TimelineStore(StringRedisTemplate redis,
                         @Value("${newsfeed.feed.max-cached-items}") int maxItems) {
        this.redis = redis;
        this.maxItems = maxItems;
    }

    private String key(String userId) {
        return KEY_PREFIX + userId;
    }

    /** Insert/update one post in a single user's timeline, then trim to bound. */
    public void push(String userId, long postId, double score) {
        String k = key(userId);
        redis.opsForZSet().add(k, Long.toString(postId), score);
        trim(k);
    }

    /**
     * Bulk-insert one post into many follower timelines. We issue per-key ZADDs;
     * for the tutorial this is clear and correct. At scale this is where you would
     * pipeline or shard the writes.
     */
    public void pushMany(List<String> userIds, long postId, double score) {
        String member = Long.toString(postId);
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

    /** Top-N (postId -> score), highest score first. */
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

    /** Remove a post from a user's timeline (used when a post is deleted). */
    public void remove(String userId, long postId) {
        redis.opsForZSet().remove(key(userId), Long.toString(postId));
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

    public List<Long> postIds(String userId, int n) {
        return new ArrayList<>(topN(userId, n).keySet());
    }
}
