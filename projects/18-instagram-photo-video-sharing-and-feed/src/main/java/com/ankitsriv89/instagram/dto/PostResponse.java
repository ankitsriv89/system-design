package com.ankitsriv89.instagram.dto;

import com.ankitsriv89.instagram.domain.Post;

import java.time.Instant;

public record PostResponse(
        Long id,
        Long userId,
        Long mediaId,
        String caption,
        Instant createdAt
) {
    public static PostResponse from(Post p) {
        return new PostResponse(p.getId(), p.getUserId(), p.getMediaId(), p.getCaption(), p.getCreatedAt());
    }
}
