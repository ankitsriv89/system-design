package com.ankitsriv89.newsfeed.dto;

import java.time.Instant;

/**
 * One entry in a rendered home feed. {@code score} is the ranking score used to
 * order the feed; {@code source} reveals whether it came from the materialized
 * (fanout-on-write) timeline or was pulled at read time (celebrity fanout-on-read).
 */
public record FeedItemResponse(
        Long postId,
        String authorId,
        String body,
        Instant createdAt,
        double score,
        String source
) {
}
