package com.ankitsriv89.instagram.service;

import com.ankitsriv89.instagram.dto.PostCreatedEvent;
import com.ankitsriv89.instagram.store.TimelineStore;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.stereotype.Service;

import java.time.Instant;
import java.util.List;

/**
 * Milestone 3 — fanout worker.
 *
 * <p>Consumes {@code post.created} and materializes the post into followers'
 * home timelines (fanout-on-write into Redis ZSETs).
 *
 * <h2>Celebrity fallback</h2>
 * Fanning out on write costs O(followerCount) Redis writes per post — the
 * celebrity write-amplification problem. Authors above
 * {@code fanout-follower-threshold} are <em>not</em> fanned out; their posts are
 * pulled at read time instead ({@link FeedService}). This hybrid keeps the write
 * path bounded while the read path stays cheap for the common case.
 *
 * <p>Idempotent: re-processing re-issues ZADDs with the same member/score (a
 * no-op), so at-least-once delivery is safe.
 */
@Service
public class FanoutService {

    private static final Logger log = LoggerFactory.getLogger(FanoutService.class);

    private final FollowService follows;
    private final TimelineStore timelines;
    private final RankingService ranking;
    private final long celebrityThreshold;

    public FanoutService(FollowService follows,
                         TimelineStore timelines,
                         RankingService ranking,
                         @Value("${instagram.feed.fanout-follower-threshold}") long celebrityThreshold) {
        this.follows = follows;
        this.timelines = timelines;
        this.ranking = ranking;
        this.celebrityThreshold = celebrityThreshold;
    }

    @KafkaListener(topics = "${instagram.kafka.topics.post-created}", groupId = "instagram-fanout")
    public void onPostCreated(PostCreatedEvent event) {
        long followerCount = follows.followerCount(event.userId());

        if (followerCount > celebrityThreshold) {
            log.info("celebrity skip user={} followers={} post={} reason=over-threshold",
                    event.userId(), followerCount, event.postId());
            return;
        }

        List<Long> followers = follows.followersOf(event.userId());
        double score = ranking.score(event.createdAt(), Instant.now());
        timelines.pushMany(followers, event.postId(), score);
        log.info("fanout-on-write user={} followers={} post={} score={}",
                event.userId(), followers.size(), event.postId(), String.format("%.4f", score));
    }
}
