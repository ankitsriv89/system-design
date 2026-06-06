package com.ankitsriv89.instagram.api;

import com.ankitsriv89.instagram.dto.CreateUploadRequest;
import com.ankitsriv89.instagram.dto.CreateUploadResponse;
import com.ankitsriv89.instagram.dto.MediaResponse;
import com.ankitsriv89.instagram.service.MediaService;
import com.ankitsriv89.instagram.service.MediaUrlService;
import jakarta.validation.Valid;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

/**
 * Milestone 1 media endpoints. Bytes never flow through here — clients PUT
 * directly to object storage via the presigned URL.
 *
 * <h2>Identity / trust boundary</h2>
 * Identity is the numeric {@code X-User-Id} header — a deliberate demo-only
 * model (seed users, no auth) so the milestone focus stays on the media
 * pipeline and feed. This header is <strong>trusted input</strong>: in any
 * deployed environment it MUST be set by an upstream gateway from a verified
 * credential (JWT/session) and never accepted from the client. Do not treat
 * this as production authentication.
 */
@RestController
@RequestMapping("/v1/media")
public class MediaController {

    private final MediaService mediaService;
    private final MediaUrlService mediaUrls;

    public MediaController(MediaService mediaService, MediaUrlService mediaUrls) {
        this.mediaService = mediaService;
        this.mediaUrls = mediaUrls;
    }

    @PostMapping("/uploads")
    public ResponseEntity<CreateUploadResponse> beginUpload(
            @RequestHeader("X-User-Id") Long userId,
            @Valid @RequestBody CreateUploadRequest request) {
        return ResponseEntity.status(HttpStatus.CREATED)
                .body(mediaService.beginUpload(userId, request.contentType()));
    }

    @PostMapping("/{mediaId}/complete")
    public MediaResponse completeUpload(
            @RequestHeader("X-User-Id") Long userId,
            @PathVariable Long mediaId) {
        return MediaResponse.from(mediaService.completeUpload(mediaId, userId), mediaUrls::urlsFor);
    }

    @GetMapping("/{mediaId}")
    public MediaResponse get(@PathVariable Long mediaId) {
        return MediaResponse.from(mediaService.get(mediaId), mediaUrls::urlsFor);
    }
}
