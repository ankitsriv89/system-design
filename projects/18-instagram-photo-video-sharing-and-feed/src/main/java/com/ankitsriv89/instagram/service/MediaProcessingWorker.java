package com.ankitsriv89.instagram.service;

import com.ankitsriv89.instagram.dto.MediaProcessedEvent;
import com.ankitsriv89.instagram.dto.MediaUploadedEvent;
import com.ankitsriv89.instagram.store.EventPublisher;
import com.ankitsriv89.instagram.store.MediaStore;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.stereotype.Component;

import java.util.LinkedHashMap;
import java.util.Map;

/**
 * Milestone 2 — variant worker.
 *
 * <p>Consumes {@code media.uploaded}. For images, generates thumbnail/small/
 * medium variants, writes them to object storage, marks the media PROCESSED, and
 * emits {@code media.processed}. Video is accepted but kept as the original only
 * (real transcoding would need ffmpeg — out of scope for this milestone).
 *
 * <p>The consumer is idempotent at the DB level: re-processing the same event
 * just overwrites the variant map and PROCESSED status.
 */
@Component
public class MediaProcessingWorker {

    private static final Logger log = LoggerFactory.getLogger(MediaProcessingWorker.class);

    private final MediaStore mediaStore;
    private final VariantGenerator variantGenerator;
    private final MediaService mediaService;
    private final EventPublisher eventPublisher;
    private final CdnInvalidationService cdnInvalidation;

    public MediaProcessingWorker(MediaStore mediaStore,
                                 VariantGenerator variantGenerator,
                                 MediaService mediaService,
                                 EventPublisher eventPublisher,
                                 CdnInvalidationService cdnInvalidation) {
        this.mediaStore = mediaStore;
        this.variantGenerator = variantGenerator;
        this.mediaService = mediaService;
        this.eventPublisher = eventPublisher;
        this.cdnInvalidation = cdnInvalidation;
    }

    @KafkaListener(topics = "${instagram.kafka.topics.media-uploaded}",
            groupId = "instagram-variant-worker")
    public void onMediaUploaded(MediaUploadedEvent event) {
        try {
            Map<String, String> variants = process(event);
            mediaService.markProcessed(event.mediaId(), variants);
            // Reprocessing rewrites variant bytes at the same keys; purge any
            // edge-cached copies so the CDN doesn't serve stale variants.
            cdnInvalidation.purge(variants.values());
            eventPublisher.publishMediaProcessed(
                    new MediaProcessedEvent(event.mediaId(), event.ownerId(), variants, true));
            log.debug("processed media_id={} variants={}", event.mediaId(), variants.keySet());
        } catch (Exception e) {
            log.error("variant generation failed for media_id={}", event.mediaId(), e);
            mediaService.markFailed(event.mediaId());
            eventPublisher.publishMediaProcessed(
                    new MediaProcessedEvent(event.mediaId(), event.ownerId(), Map.of(), false));
        }
    }

    private Map<String, String> process(MediaUploadedEvent event) {
        Map<String, String> variantKeys = new LinkedHashMap<>();
        // The original is always addressable.
        variantKeys.put("original", event.objectKey());

        if (!variantGenerator.isImage(event.contentType())) {
            // Video: keep original only for this milestone.
            return variantKeys;
        }

        byte[] original = mediaStore.getObject(event.objectKey());
        Map<String, byte[]> generated = variantGenerator.generate(original);
        generated.forEach((name, bytes) -> {
            String key = "variants/" + event.mediaId() + "/" + name + ".jpg";
            mediaStore.putObject(key, bytes, "image/jpeg");
            variantKeys.put(name, key);
        });
        return variantKeys;
    }
}
