package com.ankitsriv89.twitter.dto;

import java.time.Instant;

/**
 * Domain event published to Kafka after a tweet is durably stored. Two
 * independent consumers fan out from it:
 * <ul>
 *   <li>the fanout worker materializes the tweet into follower home timelines;</li>
 *   <li>the search indexer writes it to OpenSearch for discovery + trends.</li>
 * </ul>
 * Carries everything both consumers need so neither has to read the tweet back.
 */
public record TweetCreatedEvent(
        Long tweetId,
        String authorId,
        String text,
        Instant createdAt
) {
}
