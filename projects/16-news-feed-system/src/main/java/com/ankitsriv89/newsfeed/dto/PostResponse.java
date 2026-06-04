package com.ankitsriv89.newsfeed.dto;

import com.ankitsriv89.newsfeed.domain.Post;

import java.time.Instant;

public record PostResponse(
        Long id,
        String authorId,
        String body,
        Instant createdAt
) {
    public static PostResponse from(Post p) {
        return new PostResponse(p.getId(), p.getAuthorId(), p.getBody(), p.getCreatedAt());
    }
}
