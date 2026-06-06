package com.ankitsriv89.twitter.metrics;

import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.core.instrument.Timer;
import org.springframework.stereotype.Component;

/**
 * Golden-signal instrumentation for the timeline system. Counters and timers are
 * registered once and shared; the Prometheus registry exposes them at
 * /actuator/prometheus with the {@code twitter_} prefix.
 */
@Component
public class TwitterMetrics {

    private final Counter tweetsCreated;
    private final Counter fanoutWrites;        // timeline entries materialized on write
    private final Counter celebritySkips;      // authors over threshold, not fanned out
    private final Counter readPathPulls;       // tweets pulled at read time
    private final Counter timelineReads;
    private final Counter cacheHits;
    private final Counter cacheMisses;
    private final Counter tweetsIndexed;       // tweets written to OpenSearch
    private final Counter searchQueries;
    private final Counter trendQueries;
    private final Timer timelineBuildTimer;
    private final Timer fanoutTimer;
    private final Timer indexTimer;

    public TwitterMetrics(MeterRegistry registry) {
        tweetsCreated = Counter.builder("twitter_tweets_created_total")
                .description("Total tweets created").register(registry);
        fanoutWrites = Counter.builder("twitter_fanout_writes_total")
                .description("Timeline entries materialized on write").register(registry);
        celebritySkips = Counter.builder("twitter_celebrity_skips_total")
                .description("Tweets skipped from fanout-on-write due to celebrity threshold").register(registry);
        readPathPulls = Counter.builder("twitter_read_path_pulls_total")
                .description("Tweets pulled at read time (fanout-on-read)").register(registry);
        timelineReads = Counter.builder("twitter_timeline_reads_total")
                .description("Home timeline read requests served").register(registry);
        cacheHits = Counter.builder("twitter_timeline_cache_hits_total")
                .description("Home timeline served from materialized Redis timeline").register(registry);
        cacheMisses = Counter.builder("twitter_timeline_cache_misses_total")
                .description("Home timeline where the Redis timeline was empty").register(registry);
        tweetsIndexed = Counter.builder("twitter_tweets_indexed_total")
                .description("Tweets indexed into OpenSearch").register(registry);
        searchQueries = Counter.builder("twitter_search_queries_total")
                .description("Full-text search queries served").register(registry);
        trendQueries = Counter.builder("twitter_trend_queries_total")
                .description("Trend aggregation queries served").register(registry);
        timelineBuildTimer = Timer.builder("twitter_timeline_build_seconds")
                .description("Latency to assemble a home timeline page").register(registry);
        fanoutTimer = Timer.builder("twitter_fanout_seconds")
                .description("Latency to fan a tweet out to follower timelines").register(registry);
        indexTimer = Timer.builder("twitter_index_seconds")
                .description("Latency to index a tweet into OpenSearch").register(registry);
    }

    public void recordTweetCreated() { tweetsCreated.increment(); }
    public void recordFanoutWrites(long n) { fanoutWrites.increment(n); }
    public void recordCelebritySkip() { celebritySkips.increment(); }
    public void recordReadPathPulls(long n) { readPathPulls.increment(n); }
    public void recordTimelineRead() { timelineReads.increment(); }
    public void recordCacheHit() { cacheHits.increment(); }
    public void recordCacheMiss() { cacheMisses.increment(); }
    public void recordTweetIndexed() { tweetsIndexed.increment(); }
    public void recordSearchQuery() { searchQueries.increment(); }
    public void recordTrendQuery() { trendQueries.increment(); }
    public Timer timelineBuildTimer() { return timelineBuildTimer; }
    public Timer fanoutTimer() { return fanoutTimer; }
    public Timer indexTimer() { return indexTimer; }
}
