package com.ankitsriv89.instagram.dto;

import java.time.Instant;
import java.util.Map;

/** One item in a rendered home feed. */
public record FeedItemResponse(
        Long postId,
        Long authorId,
        String caption,
        Long mediaId,
        Map<String, String> mediaVariants,
        long likeCount,
        Instant createdAt,
        double score
) {
}
