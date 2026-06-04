package com.ankitsriv89.newsfeed.metrics;

import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.core.instrument.Timer;
import org.springframework.stereotype.Component;

/**
 * Golden-signal instrumentation for the feed system. Counters and timers are
 * registered once and shared; the Prometheus registry exposes them at
 * /actuator/prometheus with the {@code newsfeed_} prefix.
 */
@Component
public class FeedMetrics {

    private final Counter postsCreated;
    private final Counter fanoutWrites;        // timeline rows materialized on write
    private final Counter celebritySkips;      // authors over threshold, not fanned out
    private final Counter readPathPulls;       // posts pulled at read time
    private final Counter feedReads;
    private final Counter cacheHits;
    private final Counter cacheMisses;
    private final Timer feedBuildTimer;
    private final Timer fanoutTimer;

    public FeedMetrics(MeterRegistry registry) {
        postsCreated = Counter.builder("newsfeed_posts_created_total")
                .description("Total posts created").register(registry);
        fanoutWrites = Counter.builder("newsfeed_fanout_writes_total")
                .description("Timeline entries materialized on write").register(registry);
        celebritySkips = Counter.builder("newsfeed_celebrity_skips_total")
                .description("Posts skipped from fanout-on-write due to celebrity threshold").register(registry);
        readPathPulls = Counter.builder("newsfeed_read_path_pulls_total")
                .description("Posts pulled at read time (fanout-on-read)").register(registry);
        feedReads = Counter.builder("newsfeed_feed_reads_total")
                .description("Home feed read requests served").register(registry);
        cacheHits = Counter.builder("newsfeed_feed_cache_hits_total")
                .description("Home feed served from materialized Redis timeline").register(registry);
        cacheMisses = Counter.builder("newsfeed_feed_cache_misses_total")
                .description("Home feed where the Redis timeline was empty").register(registry);
        feedBuildTimer = Timer.builder("newsfeed_feed_build_seconds")
                .description("Latency to assemble a home feed page").register(registry);
        fanoutTimer = Timer.builder("newsfeed_fanout_seconds")
                .description("Latency to fan a post out to follower timelines").register(registry);
    }

    public void recordPostCreated() { postsCreated.increment(); }
    public void recordFanoutWrites(long n) { fanoutWrites.increment(n); }
    public void recordCelebritySkip() { celebritySkips.increment(); }
    public void recordReadPathPulls(long n) { readPathPulls.increment(n); }
    public void recordFeedRead() { feedReads.increment(); }
    public void recordCacheHit() { cacheHits.increment(); }
    public void recordCacheMiss() { cacheMisses.increment(); }
    public Timer feedBuildTimer() { return feedBuildTimer; }
    public Timer fanoutTimer() { return fanoutTimer; }
}
