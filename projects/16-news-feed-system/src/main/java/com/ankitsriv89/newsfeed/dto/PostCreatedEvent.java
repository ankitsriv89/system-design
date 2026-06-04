package com.ankitsriv89.newsfeed.dto;

import java.time.Instant;

/**
 * Domain event published to Kafka after a post is durably stored. The fanout
 * worker consumes this to materialize the post into follower timelines.
 * Carries everything the worker needs so it never has to read back the post.
 */
public record PostCreatedEvent(
        Long postId,
        String authorId,
        String body,
        Instant createdAt
) {
}
