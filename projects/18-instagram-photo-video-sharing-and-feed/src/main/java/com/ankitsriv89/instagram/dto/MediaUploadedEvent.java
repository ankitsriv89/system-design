package com.ankitsriv89.instagram.dto;

/**
 * Emitted to Kafka topic {@code media.uploaded} once a client confirms the
 * original bytes are in object storage. The variant worker (milestone 2)
 * consumes this to generate thumbnail/small/medium variants.
 */
public record MediaUploadedEvent(
        Long mediaId,
        Long ownerId,
        String objectKey,
        String contentType
) {
}
