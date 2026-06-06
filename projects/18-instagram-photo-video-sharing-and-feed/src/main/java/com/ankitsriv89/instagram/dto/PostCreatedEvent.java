package com.ankitsriv89.instagram.dto;

import java.time.Instant;

/**
 * Emitted to {@code post.created}. The fanout worker consumes it to materialize
 * the post into followers' home timelines (push), unless the author is a
 * celebrity (pulled at read time instead).
 */
public record PostCreatedEvent(
        Long postId,
        Long userId,
        Instant createdAt
) {
}
