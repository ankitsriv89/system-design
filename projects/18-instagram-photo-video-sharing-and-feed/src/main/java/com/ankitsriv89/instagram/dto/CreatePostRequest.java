package com.ankitsriv89.instagram.dto;

import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;

public record CreatePostRequest(
        @NotNull Long mediaId,
        @Size(max = 2200) String caption
) {
}
