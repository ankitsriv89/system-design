package com.ankitsriv89.instagram.dto;

/** Emitted to {@code post.liked} when a user likes (or unlikes) a post. */
public record PostLikedEvent(
        Long postId,
        Long userId,
        boolean liked
) {
}
