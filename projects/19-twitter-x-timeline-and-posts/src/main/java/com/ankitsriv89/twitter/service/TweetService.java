package com.ankitsriv89.twitter.service;

import com.ankitsriv89.twitter.domain.Tweet;
import com.ankitsriv89.twitter.dto.TweetCreatedEvent;
import com.ankitsriv89.twitter.metrics.TwitterMetrics;
import com.ankitsriv89.twitter.repository.TweetRepository;
import com.ankitsriv89.twitter.store.EventPublisher;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;

import java.time.Instant;
import java.util.Optional;

/**
 * Write path for tweets. A tweet is committed to PostgreSQL first; the
 * tweet.created event is published to Kafka only <em>after</em> the DB
 * transaction commits. This ordering guarantees we never fan out or index a
 * tweet that wasn't durably stored — the source of truth leads, the async
 * pipeline (fanout + search indexing) follows.
 */
@Service
public class TweetService {

    private final TweetRepository tweets;
    private final EventPublisher events;
    private final TwitterMetrics metrics;

    public TweetService(TweetRepository tweets, EventPublisher events, TwitterMetrics metrics) {
        this.tweets = tweets;
        this.events = events;
        this.metrics = metrics;
    }

    @Transactional
    public Tweet create(String authorId, String text) {
        Tweet saved = tweets.save(new Tweet(authorId, text, Instant.now()));
        TweetCreatedEvent event = new TweetCreatedEvent(
                saved.getId(), saved.getAuthorId(), saved.getText(), saved.getCreatedAt());

        // After-commit callback: the event fires only once the row is durable.
        // If the transaction rolls back, no event is published.
        TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
            @Override
            public void afterCommit() {
                events.publishTweetCreated(event);
            }
        });

        metrics.recordTweetCreated();
        return saved;
    }

    @Transactional
    public void delete(String requesterId, long tweetId) {
        Optional<Tweet> found = tweets.findById(tweetId);
        if (found.isEmpty()) {
            return;
        }
        Tweet tweet = found.get();
        if (!tweet.getAuthorId().equals(requesterId)) {
            throw new SecurityException("only the author can delete this tweet");
        }
        // Soft delete: the row stays for audit; the read path filters it out and
        // the timeline assembler drops it. Deletes are honored lazily at read
        // time even if a stale Redis timeline entry lingers.
        tweet.setDeleted(true);
        tweets.save(tweet);
    }

    public Optional<Tweet> find(long tweetId) {
        return tweets.findById(tweetId);
    }
}
