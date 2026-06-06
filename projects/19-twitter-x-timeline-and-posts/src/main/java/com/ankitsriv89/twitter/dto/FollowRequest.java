package com.ankitsriv89.twitter.dto;

import jakarta.validation.constraints.NotBlank;

public record FollowRequest(
        @NotBlank String followeeId
) {
}
