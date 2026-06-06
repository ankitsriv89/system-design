package com.ankitsriv89.twitter.service;

import com.ankitsriv89.twitter.dto.TweetCreatedEvent;
import com.ankitsriv89.twitter.metrics.TwitterMetrics;
import com.ankitsriv89.twitter.store.TimelineStore;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.stereotype.Service;

import java.time.Instant;
import java.util.List;

/**
 * The fanout worker. Consumes {@code tweet.created} events off Kafka and
 * materializes each tweet into its followers' home timelines (fanout-on-write).
 *
 * <h2>Celebrity fallback</h2>
 * Fanning out on write costs O(followerCount) Redis writes per tweet. For an
 * account with millions of followers this is catastrophic write amplification —
 * the "celebrity fanout" failure mode. So authors whose follower count exceeds
 * {@code celebrity-threshold} are <em>not</em> fanned out here; their tweets are
 * pulled at read time instead (see {@link TimelineService}). This hybrid keeps
 * the write path bounded while keeping the read path cheap for the common case.
 *
 * <p>Idempotent: re-processing the same event re-issues ZADDs with the same
 * member and score, a no-op. That makes at-least-once Kafka delivery safe.
 */
@Service
public class FanoutService {

    private static final Logger log = LoggerFactory.getLogger(FanoutService.class);

    private final FollowService follows;
    private final TimelineStore timelines;
    private final RankingService ranking;
    private final TwitterMetrics metrics;
    private final long celebrityThreshold;

    public FanoutService(FollowService follows,
                         TimelineStore timelines,
                         RankingService ranking,
                         TwitterMetrics metrics,
                         @Value("${twitter.fanout.celebrity-threshold}") long celebrityThreshold) {
        this.follows = follows;
        this.timelines = timelines;
        this.ranking = ranking;
        this.metrics = metrics;
        this.celebrityThreshold = celebrityThreshold;
    }

    @KafkaListener(topics = "${twitter.kafka.topic.tweets}", groupId = "twitter-fanout")
    public void onTweetCreated(TweetCreatedEvent event) {
        metrics.fanoutTimer().record(() -> fanout(event));
    }

    private void fanout(TweetCreatedEvent event) {
        long followerCount = follows.followerCount(event.authorId());

        if (followerCount > celebrityThreshold) {
            // Celebrity: skip write fanout. The read path pulls this author's
            // tweets on demand. Recorded so the tradeoff is observable.
            metrics.recordCelebritySkip();
            log.info("celebrity skip author={} followers={} tweetId={} reason=over-threshold",
                    event.authorId(), followerCount, event.tweetId());
            return;
        }

        List<String> followers = follows.followersOf(event.authorId());
        double score = ranking.score(event.createdAt(), Instant.now());
        timelines.pushMany(followers, event.tweetId(), score);
        metrics.recordFanoutWrites(followers.size());
        log.info("fanout-on-write author={} followers={} tweetId={} score={}",
                event.authorId(), followers.size(), event.tweetId(), String.format("%.4f", score));
    }
}
