package com.ankitsriv89.instagram.domain;

/**
 * Lifecycle of a media object.
 *
 * <p>PENDING: original uploaded to object storage, variants not yet generated.
 * PROCESSED: variant worker finished; thumbnail/small/medium ready.
 * FAILED: transcoding/variant generation failed (see build milestone 2).
 */
public enum MediaStatus {
    PENDING,
    PROCESSED,
    FAILED
}
