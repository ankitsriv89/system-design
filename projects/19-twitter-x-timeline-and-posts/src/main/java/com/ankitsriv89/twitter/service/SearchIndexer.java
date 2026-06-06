package com.ankitsriv89.twitter.service;

import com.ankitsriv89.twitter.dto.TweetCreatedEvent;
import com.ankitsriv89.twitter.metrics.TwitterMetrics;
import com.ankitsriv89.twitter.store.SearchStore;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.stereotype.Service;

/**
 * The search-indexing worker — a second, independent consumer group on the same
 * {@code tweet.created} stream as the fanout worker. It writes each tweet into
 * the OpenSearch {@code tweets} index, which backs both full-text search and
 * trend aggregation.
 *
 * <p>Separating this from fanout means a slow/failed search index never blocks
 * timeline delivery, and vice versa — "search lag" degrades discovery without
 * touching the home timeline. Indexing is idempotent (doc id = tweetId), so
 * at-least-once redelivery is safe.
 */
@Service
public class SearchIndexer {

    private static final Logger log = LoggerFactory.getLogger(SearchIndexer.class);

    private final SearchStore search;
    private final TwitterMetrics metrics;

    public SearchIndexer(SearchStore search, TwitterMetrics metrics) {
        this.search = search;
        this.metrics = metrics;
    }

    @KafkaListener(topics = "${twitter.kafka.topic.tweets}", groupId = "twitter-search-indexer")
    public void onTweetCreated(TweetCreatedEvent event) {
        try {
            metrics.indexTimer().recordCallable(() -> {
                search.index(event.tweetId(), event.authorId(), event.text(), event.createdAt());
                return null;
            });
            metrics.recordTweetIndexed();
            log.info("indexed tweetId={} author={}", event.tweetId(), event.authorId());
        } catch (Exception e) {
            // Throwing lets the container retry per its error handler. We log so
            // search lag is observable rather than silent.
            log.error("failed to index tweetId={}: {}", event.tweetId(), e.getMessage());
            throw new RuntimeException(e);
        }
    }
}
