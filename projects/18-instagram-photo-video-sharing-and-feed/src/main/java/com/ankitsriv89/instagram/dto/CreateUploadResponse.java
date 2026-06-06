package com.ankitsriv89.instagram.dto;

/**
 * Response to an upload request. The client PUTs the raw bytes to
 * {@code uploadUrl} (direct to object storage), then calls the complete
 * endpoint with {@code mediaId}.
 */
public record CreateUploadResponse(
        Long mediaId,
        String objectKey,
        String uploadUrl
) {
}
