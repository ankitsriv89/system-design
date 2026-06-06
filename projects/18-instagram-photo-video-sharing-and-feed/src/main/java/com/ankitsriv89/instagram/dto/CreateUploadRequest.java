package com.ankitsriv89.instagram.dto;

import jakarta.validation.constraints.NotBlank;

/** Request to begin an upload: the client declares the content type up front. */
public record CreateUploadRequest(
        @NotBlank String contentType
) {
}
