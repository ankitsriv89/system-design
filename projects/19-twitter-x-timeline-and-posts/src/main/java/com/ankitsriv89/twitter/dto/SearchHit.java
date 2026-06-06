package com.ankitsriv89.twitter.dto;

import java.time.Instant;

/** One full-text search result from the OpenSearch tweets index. */
public record SearchHit(
        Long tweetId,
        String authorId,
        String text,
        Instant createdAt,
        double relevance
) {
}
