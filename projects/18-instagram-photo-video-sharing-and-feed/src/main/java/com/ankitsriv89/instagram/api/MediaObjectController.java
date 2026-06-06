package com.ankitsriv89.instagram.api;

import com.ankitsriv89.instagram.store.MediaStore;
import org.springframework.http.CacheControl;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.servlet.HandlerMapping;

import jakarta.servlet.http.HttpServletRequest;

import java.time.Duration;

/**
 * Milestone 4 — local CDN-shaped media serving.
 *
 * <p>Serves object bytes from MinIO at the same URL shape the CDN uses
 * ({@code public-base-url}/{objectKey}). Behind Caddy the public path is
 * {@code /p18/media/**}; Caddy strips {@code /p18/} so this controller sees
 * {@code /media/**}. In production these requests terminate at Cloudflare's edge
 * and only miss through to this origin.
 *
 * <p>Variants are immutable (unique keys), so they get a long, immutable
 * Cache-Control. The cache-invalidation seam ({@code CdnInvalidationService})
 * handles the reprocess/delete cases where a key's bytes do change.
 */
@RestController
@RequestMapping("/media")
public class MediaObjectController {

    private final MediaStore mediaStore;

    public MediaObjectController(MediaStore mediaStore) {
        this.mediaStore = mediaStore;
    }

    @GetMapping("/**")
    public ResponseEntity<byte[]> serve(HttpServletRequest request) {
        String fullPath = (String) request.getAttribute(
                HandlerMapping.PATH_WITHIN_HANDLER_MAPPING_ATTRIBUTE);
        // strip the leading "/media/" to recover the object key
        String objectKey = fullPath.replaceFirst("^/media/", "");

        if (!mediaStore.objectExists(objectKey)) {
            return ResponseEntity.notFound().build();
        }
        byte[] bytes = mediaStore.getObject(objectKey);
        return ResponseEntity.ok()
                .cacheControl(CacheControl.maxAge(Duration.ofDays(365)).cachePublic().immutable())
                .contentType(guessContentType(objectKey))
                .body(bytes);
    }

    private MediaType guessContentType(String key) {
        String lower = key.toLowerCase();
        if (lower.endsWith(".jpg") || lower.endsWith(".jpeg")) return MediaType.IMAGE_JPEG;
        if (lower.endsWith(".png")) return MediaType.IMAGE_PNG;
        if (lower.endsWith(".gif")) return MediaType.IMAGE_GIF;
        return MediaType.APPLICATION_OCTET_STREAM;
    }
}
