package com.ankitsriv89.instagram.dto;

import com.ankitsriv89.instagram.domain.Media;
import com.ankitsriv89.instagram.domain.MediaStatus;

import java.util.Map;
import java.util.function.UnaryOperator;

/** Public view of a media object. {@code variants} maps variant name -> URL. */
public record MediaResponse(
        Long id,
        Long ownerId,
        String objectKey,
        String contentType,
        MediaStatus status,
        Map<String, String> variants
) {
    /**
     * Build a response, converting variant object keys to public URLs via the
     * supplied mapper (typically MediaUrlService::urlsFor).
     */
    public static MediaResponse from(Media m, UnaryOperator<Map<String, String>> toUrls) {
        return new MediaResponse(
                m.getId(), m.getOwnerId(), m.getObjectKey(),
                m.getContentType(), m.getStatus(), toUrls.apply(m.getVariants()));
    }
}
