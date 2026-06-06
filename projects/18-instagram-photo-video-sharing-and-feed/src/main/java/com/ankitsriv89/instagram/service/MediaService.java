package com.ankitsriv89.instagram.service;

import com.ankitsriv89.instagram.domain.Media;
import com.ankitsriv89.instagram.domain.MediaStatus;
import com.ankitsriv89.instagram.dto.CreateUploadResponse;
import com.ankitsriv89.instagram.dto.MediaUploadedEvent;
import com.ankitsriv89.instagram.repository.MediaRepository;
import com.ankitsriv89.instagram.store.EventPublisher;
import com.ankitsriv89.instagram.store.MediaStore;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.UUID;

/**
 * Milestone 1 — upload + metadata.
 *
 * <p>Two-phase upload: {@link #beginUpload} creates a PENDING metadata row and
 * returns a presigned PUT URL; the client uploads bytes directly to object
 * storage; {@link #completeUpload} verifies the object landed and emits
 * {@code media.uploaded} for the variant worker (milestone 2).
 */
@Service
public class MediaService {

    private static final Logger log = LoggerFactory.getLogger(MediaService.class);

    private final MediaRepository mediaRepository;
    private final MediaStore mediaStore;
    private final EventPublisher eventPublisher;

    public MediaService(MediaRepository mediaRepository,
                        MediaStore mediaStore,
                        EventPublisher eventPublisher) {
        this.mediaRepository = mediaRepository;
        this.mediaStore = mediaStore;
        this.eventPublisher = eventPublisher;
    }

    @Transactional
    public CreateUploadResponse beginUpload(Long ownerId, String contentType) {
        String objectKey = "originals/" + ownerId + "/" + UUID.randomUUID();
        Media media = mediaRepository.save(new Media(ownerId, objectKey, contentType));
        String uploadUrl = mediaStore.presignedPutUrl(objectKey);
        log.debug("begin upload media_id={} owner={} key={}", media.getId(), ownerId, objectKey);
        return new CreateUploadResponse(media.getId(), objectKey, uploadUrl);
    }

    @Transactional
    public Media completeUpload(Long mediaId, Long ownerId) {
        Media media = mediaRepository.findById(mediaId)
                .orElseThrow(() -> new IllegalArgumentException("media not found: " + mediaId));
        if (!media.getOwnerId().equals(ownerId)) {
            throw new SecurityException("media " + mediaId + " not owned by user " + ownerId);
        }
        if (!mediaStore.objectExists(media.getObjectKey())) {
            throw new IllegalStateException("object not uploaded yet: " + media.getObjectKey());
        }
        // Stays PENDING until the variant worker marks it PROCESSED (milestone 2).
        eventPublisher.publishMediaUploaded(new MediaUploadedEvent(
                media.getId(), media.getOwnerId(), media.getObjectKey(), media.getContentType()));
        log.debug("completed upload media_id={} -> emitted media.uploaded", mediaId);
        return media;
    }

    @Transactional(readOnly = true)
    public Media get(Long mediaId) {
        return mediaRepository.findById(mediaId)
                .orElseThrow(() -> new IllegalArgumentException("media not found: " + mediaId));
    }

    /** Used by the variant worker (milestone 2) to record results. */
    @Transactional
    public void markProcessed(Long mediaId, java.util.Map<String, String> variants) {
        Media media = get(mediaId);
        media.setVariants(variants);
        media.setStatus(MediaStatus.PROCESSED);
        mediaRepository.save(media);
    }

    @Transactional
    public void markFailed(Long mediaId) {
        Media media = get(mediaId);
        media.setStatus(MediaStatus.FAILED);
        mediaRepository.save(media);
    }
}
