package com.ankitsriv89.twitter.dto;

import com.ankitsriv89.twitter.domain.Tweet;

import java.time.Instant;

public record TweetResponse(
        Long id,
        String authorId,
        String text,
        Instant createdAt
) {
    public static TweetResponse from(Tweet t) {
        return new TweetResponse(t.getId(), t.getAuthorId(), t.getText(), t.getCreatedAt());
    }
}
