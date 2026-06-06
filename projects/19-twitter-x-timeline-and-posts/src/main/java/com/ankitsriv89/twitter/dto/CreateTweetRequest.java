package com.ankitsriv89.twitter.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public record CreateTweetRequest(
        @NotBlank @Size(max = 280) String text
) {
}
