package com.ankitsriv89.twitter.dto;

import java.time.Instant;

/**
 * One entry in a rendered home timeline. {@code score} is the ranking score used
 * to order the timeline; {@code source} reveals whether it came from the
 * materialized (fanout-on-write) timeline or was pulled at read time
 * (celebrity fanout-on-read).
 */
public record TimelineItemResponse(
        Long tweetId,
        String authorId,
        String text,
        Instant createdAt,
        double score,
        String source
) {
}
