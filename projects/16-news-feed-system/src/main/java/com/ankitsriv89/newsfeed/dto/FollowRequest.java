package com.ankitsriv89.newsfeed.dto;

import jakarta.validation.constraints.NotBlank;

public record FollowRequest(
        @NotBlank String followeeId
) {
}
