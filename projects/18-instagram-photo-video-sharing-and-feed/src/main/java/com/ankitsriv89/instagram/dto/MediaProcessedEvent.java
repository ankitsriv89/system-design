package com.ankitsriv89.instagram.dto;

import java.util.Map;

/**
 * Emitted to {@code media.processed} after the variant worker generates (or
 * skips, for video) variants. {@code variants} maps variant name -> object key.
 */
public record MediaProcessedEvent(
        Long mediaId,
        Long ownerId,
        Map<String, String> variants,
        boolean success
) {
}
