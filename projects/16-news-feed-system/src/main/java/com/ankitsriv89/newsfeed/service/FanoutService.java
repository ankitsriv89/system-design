package com.ankitsriv89.newsfeed.service;

import com.ankitsriv89.newsfeed.dto.PostCreatedEvent;
import com.ankitsriv89.newsfeed.metrics.FeedMetrics;
import com.ankitsriv89.newsfeed.store.TimelineStore;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.stereotype.Service;

import java.time.Instant;
import java.util.List;

/**
 * The fanout worker. Consumes {@code post.created} events off Kafka and
 * materializes each post into its followers' home timelines (fanout-on-write).
 *
 * <h2>Celebrity fallback</h2>
 * Fanning out on write costs O(followerCount) Redis writes per post. For a user
 * with millions of followers this is catastrophic write amplification — the
 * "celebrity fanout" failure mode. So authors whose follower count exceeds
 * {@code celebrity-threshold} are <em>not</em> fanned out here; their posts are
 * pulled at read time instead (see {@link FeedService}). This hybrid keeps the
 * write path bounded while keeping the read path cheap for the common case.
 *
 * <p>The work is idempotent: re-processing the same event re-issues ZADDs with
 * the same member and score, which is a no-op. That makes at-least-once Kafka
 * delivery safe.
 */
@Service
public class FanoutService {

    private static final Logger log = LoggerFactory.getLogger(FanoutService.class);

    private final FollowService follows;
    private final TimelineStore timelines;
    private final RankingService ranking;
    private final FeedMetrics metrics;
    private final long celebrityThreshold;

    public FanoutService(FollowService follows,
                         TimelineStore timelines,
                         RankingService ranking,
                         FeedMetrics metrics,
                         @Value("${newsfeed.fanout.celebrity-threshold}") long celebrityThreshold) {
        this.follows = follows;
        this.timelines = timelines;
        this.ranking = ranking;
        this.metrics = metrics;
        this.celebrityThreshold = celebrityThreshold;
    }

    @KafkaListener(topics = "${newsfeed.kafka.topic.events}", groupId = "newsfeed-fanout")
    public void onPostCreated(PostCreatedEvent event) {
        metrics.fanoutTimer().record(() -> fanout(event));
    }

    private void fanout(PostCreatedEvent event) {
        long followerCount = follows.followerCount(event.authorId());

        if (followerCount > celebrityThreshold) {
            // Celebrity: skip write fanout. Read path will pull this author's
            // posts on demand. We record the skip so the tradeoff is observable.
            metrics.recordCelebritySkip();
            log.info("celebrity skip author={} followers={} postId={} reason=over-threshold",
                    event.authorId(), followerCount, event.postId());
            return;
        }

        List<String> followers = follows.followersOf(event.authorId());
        double score = ranking.score(event.createdAt(), Instant.now());
        timelines.pushMany(followers, event.postId(), score);
        metrics.recordFanoutWrites(followers.size());
        log.info("fanout-on-write author={} followers={} postId={} score={}",
                event.authorId(), followers.size(), event.postId(), String.format("%.4f", score));
    }
}
