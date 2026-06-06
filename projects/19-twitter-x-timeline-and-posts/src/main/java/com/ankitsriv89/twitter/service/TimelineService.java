package com.ankitsriv89.twitter.service;

import com.ankitsriv89.twitter.domain.Tweet;
import com.ankitsriv89.twitter.dto.TimelineItemResponse;
import com.ankitsriv89.twitter.metrics.TwitterMetrics;
import com.ankitsriv89.twitter.repository.TweetRepository;
import com.ankitsriv89.twitter.store.TimelineStore;
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
 * The read path. Assembling a home timeline is a merge of two sources:
 *
 * <ol>
 *   <li><b>Materialized timeline</b> — tweet IDs pushed to {@code timeline:{userId}}
 *       by the fanout worker for non-celebrity authors. Cheap: one Redis read.</li>
 *   <li><b>Read-time pull</b> — for celebrity authors the user follows (those
 *       skipped on write), pull their recent tweets straight from PostgreSQL.</li>
 * </ol>
 *
 * The two sets are merged, de-duplicated, re-scored with the current ranking,
 * soft-deleted tweets are dropped, and the top page is returned. This is the
 * hybrid fanout that keeps both write and read paths bounded.
 */
@Service
public class TimelineService {

    private final TimelineStore timelines;
    private final TweetRepository tweets;
    private final FollowService follows;
    private final RankingService ranking;
    private final TwitterMetrics metrics;
    private final long celebrityThreshold;
    private final int pageSize;

    public TimelineService(TimelineStore timelines,
                           TweetRepository tweets,
                           FollowService follows,
                           RankingService ranking,
                           TwitterMetrics metrics,
                           @Value("${twitter.fanout.celebrity-threshold}") long celebrityThreshold,
                           @Value("${twitter.timeline.page-size}") int pageSize) {
        this.timelines = timelines;
        this.tweets = tweets;
        this.follows = follows;
        this.ranking = ranking;
        this.metrics = metrics;
        this.celebrityThreshold = celebrityThreshold;
        this.pageSize = pageSize;
    }

    public List<TimelineItemResponse> homeTimeline(String userId, int limit) {
        return metrics.timelineBuildTimer().record(() -> buildTimeline(userId, limit));
    }

    private List<TimelineItemResponse> buildTimeline(String userId, int limit) {
        metrics.recordTimelineRead();
        int pageLimit = limit > 0 ? limit : pageSize;
        Instant now = Instant.now();

        // --- Source 1: materialized timeline (fanout-on-write) ---
        // Over-fetch so dropping deleted tweets still leaves a full page.
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

        // Merge candidate tweet IDs. Resolve each to its Tweet row so we can
        // (a) drop soft-deleted tweets and (b) re-score against "now".
        Map<Long, TimelineItemResponse> merged = new LinkedHashMap<>();

        if (!materialized.isEmpty()) {
            List<Tweet> rows = tweets.findAllById(materialized.keySet());
            for (Tweet t : rows) {
                if (t.isDeleted()) {
                    continue;   // honor deletes lazily even if a stale ZSET entry lingers
                }
                double score = ranking.score(t, now);
                merged.put(t.getId(), new TimelineItemResponse(
                        t.getId(), t.getAuthorId(), t.getText(), t.getCreatedAt(), score, "materialized"));
            }
        }

        if (!celebrities.isEmpty()) {
            List<Tweet> celebTweets = tweets.findRecentByAuthors(
                    celebrities, PageRequest.of(0, pageLimit * 2));
            long pulled = 0;
            for (Tweet t : celebTweets) {
                if (t.isDeleted() || merged.containsKey(t.getId())) {
                    continue;
                }
                double score = ranking.score(t, now);
                merged.put(t.getId(), new TimelineItemResponse(
                        t.getId(), t.getAuthorId(), t.getText(), t.getCreatedAt(), score, "read-path"));
                pulled++;
            }
            metrics.recordReadPathPulls(pulled);
        }

        List<TimelineItemResponse> result = new ArrayList<>(merged.values());
        result.sort(Comparator.comparingDouble(TimelineItemResponse::score).reversed());
        if (result.size() > pageLimit) {
            return result.subList(0, pageLimit);
        }
        return result;
    }

    /**
     * Backfill: reconstruct a user's materialized timeline from PostgreSQL. Used
     * when a follow is added (so the new followee's history appears) or to repair
     * a timeline after Redis loss. Pulls recent non-celebrity followee tweets and
     * rewrites the ZSET.
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
        List<Tweet> recent = tweets.findRecentByAuthors(followees, PageRequest.of(0, maxItems));
        Map<Long, Double> scored = new LinkedHashMap<>();
        for (Tweet t : recent) {
            if (!t.isDeleted()) {
                scored.put(t.getId(), ranking.score(t, now));
            }
        }
        timelines.replace(userId, scored);
        return scored.size();
    }

    public long timelineSize(String userId) {
        return timelines.size(userId);
    }
}
