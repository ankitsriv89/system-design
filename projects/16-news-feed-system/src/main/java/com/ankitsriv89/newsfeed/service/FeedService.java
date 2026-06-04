package com.ankitsriv89.newsfeed.service;

import com.ankitsriv89.newsfeed.domain.Post;
import com.ankitsriv89.newsfeed.dto.FeedItemResponse;
import com.ankitsriv89.newsfeed.metrics.FeedMetrics;
import com.ankitsriv89.newsfeed.repository.PostRepository;
import com.ankitsriv89.newsfeed.store.TimelineStore;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.domain.PageRequest;
import org.springframework.stereotype.Service;

import java.time.Instant;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * The read path. Assembling a home feed is a merge of two sources:
 *
 * <ol>
 *   <li><b>Materialized timeline</b> — post IDs pushed to {@code feed:{userId}}
 *       by the fanout worker for non-celebrity authors. Cheap: one Redis read.</li>
 *   <li><b>Read-time pull</b> — for celebrity authors the user follows (those
 *       skipped on write), pull their recent posts straight from PostgreSQL.</li>
 * </ol>
 *
 * The two sets are merged, de-duplicated, re-scored with the current ranking,
 * soft-deleted posts are dropped, and the top page is returned. This is the
 * hybrid fanout that keeps both write and read paths bounded.
 */
@Service
public class FeedService {

    private final TimelineStore timelines;
    private final PostRepository posts;
    private final FollowService follows;
    private final RankingService ranking;
    private final FeedMetrics metrics;
    private final long celebrityThreshold;
    private final int pageSize;

    public FeedService(TimelineStore timelines,
                       PostRepository posts,
                       FollowService follows,
                       RankingService ranking,
                       FeedMetrics metrics,
                       @Value("${newsfeed.fanout.celebrity-threshold}") long celebrityThreshold,
                       @Value("${newsfeed.feed.page-size}") int pageSize) {
        this.timelines = timelines;
        this.posts = posts;
        this.follows = follows;
        this.ranking = ranking;
        this.metrics = metrics;
        this.celebrityThreshold = celebrityThreshold;
        this.pageSize = pageSize;
    }

    public List<FeedItemResponse> homeFeed(String userId, int limit) {
        return metrics.feedBuildTimer().record(() -> buildFeed(userId, limit));
    }

    private List<FeedItemResponse> buildFeed(String userId, int limit) {
        metrics.recordFeedRead();
        int pageLimit = limit > 0 ? limit : pageSize;
        Instant now = Instant.now();

        // --- Source 1: materialized timeline (fanout-on-write) ---
        // Over-fetch so that dropping deleted posts still leaves a full page.
        Map<Long, Double> materialized = timelines.topN(userId, pageLimit * 3);
        if (materialized.isEmpty()) {
            metrics.recordCacheMiss();
        } else {
            metrics.recordCacheHit();
        }

        // --- Source 2: read-time pull for celebrity authors (fanout-on-read) ---
        List<String> celebrities = follows.followeesOf(userId).stream()
                .filter(a -> follows.followerCount(a) > celebrityThreshold)
                .toList();

        // Merge candidate post IDs. We resolve every candidate to its Post row so
        // we can (a) drop soft-deleted posts and (b) re-score against "now".
        Map<Long, FeedItemResponse> merged = new LinkedHashMap<>();

        // Materialized candidates → fetch rows in one query.
        if (!materialized.isEmpty()) {
            List<Post> rows = posts.findAllById(materialized.keySet());
            for (Post p : rows) {
                if (p.isDeleted()) {
                    continue;   // honor deletes lazily even if a stale ZSET entry lingers
                }
                double score = ranking.score(p, now);
                merged.put(p.getId(), new FeedItemResponse(
                        p.getId(), p.getAuthorId(), p.getBody(), p.getCreatedAt(), score, "materialized"));
            }
        }

        // Celebrity candidates → pull recent posts directly from the source store.
        if (!celebrities.isEmpty()) {
            List<Post> celebPosts = posts.findRecentByAuthors(
                    celebrities, PageRequest.of(0, pageLimit * 2));
            long pulled = 0;
            for (Post p : celebPosts) {
                if (p.isDeleted() || merged.containsKey(p.getId())) {
                    continue;
                }
                double score = ranking.score(p, now);
                merged.put(p.getId(), new FeedItemResponse(
                        p.getId(), p.getAuthorId(), p.getBody(), p.getCreatedAt(), score, "read-path"));
                pulled++;
            }
            metrics.recordReadPathPulls(pulled);
        }

        // Final ranked page, highest score first.
        List<FeedItemResponse> result = new ArrayList<>(merged.values());
        result.sort(Comparator.comparingDouble(FeedItemResponse::score).reversed());
        if (result.size() > pageLimit) {
            return result.subList(0, pageLimit);
        }
        return result;
    }

    /**
     * Backfill: reconstruct a user's materialized timeline from the source store.
     * Used when a follow is added (so the new followee's history appears) or to
     * repair a timeline after Redis loss. Pulls recent non-celebrity followee
     * posts and rewrites the ZSET.
     */
    public int backfill(String userId, int maxItems) {
        Instant now = Instant.now();
        List<String> followees = follows.followeesOf(userId).stream()
                .filter(a -> follows.followerCount(a) <= celebrityThreshold)
                .toList();
        if (followees.isEmpty()) {
            timelines.replace(userId, Map.of());
            return 0;
        }
        List<Post> recent = posts.findRecentByAuthors(followees, PageRequest.of(0, maxItems));
        Map<Long, Double> scored = new LinkedHashMap<>();
        for (Post p : recent) {
            if (!p.isDeleted()) {
                scored.put(p.getId(), ranking.score(p, now));
            }
        }
        timelines.replace(userId, scored);
        return scored.size();
    }

    public long timelineSize(String userId) {
        return timelines.size(userId);
    }
}
