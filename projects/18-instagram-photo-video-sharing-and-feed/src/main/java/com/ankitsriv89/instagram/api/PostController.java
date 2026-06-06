package com.ankitsriv89.instagram.api;

import com.ankitsriv89.instagram.dto.CreatePostRequest;
import com.ankitsriv89.instagram.dto.PostResponse;
import com.ankitsriv89.instagram.service.EngagementService;
import com.ankitsriv89.instagram.service.PostService;
import jakarta.validation.Valid;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.Map;

/**
 * Posts and their likes. {@code X-User-Id} is the trusted demo identity header
 * (see {@link MediaController} for the trust-boundary note).
 */
@RestController
@RequestMapping("/v1/posts")
public class PostController {

    private final PostService postService;
    private final EngagementService engagementService;

    public PostController(PostService postService, EngagementService engagementService) {
        this.postService = postService;
        this.engagementService = engagementService;
    }

    @PostMapping
    public ResponseEntity<PostResponse> create(
            @RequestHeader("X-User-Id") Long userId,
            @Valid @RequestBody CreatePostRequest request) {
        return ResponseEntity.status(HttpStatus.CREATED).body(
                PostResponse.from(postService.createPost(userId, request.mediaId(), request.caption())));
    }

    @GetMapping("/{postId}")
    public PostResponse get(@PathVariable Long postId) {
        return PostResponse.from(postService.get(postId));
    }

    @PostMapping("/{postId}/likes")
    public Map<String, Long> like(
            @RequestHeader("X-User-Id") Long userId,
            @PathVariable Long postId) {
        return Map.of("likeCount", engagementService.like(postId, userId));
    }

    @DeleteMapping("/{postId}/likes")
    public Map<String, Long> unlike(
            @RequestHeader("X-User-Id") Long userId,
            @PathVariable Long postId) {
        return Map.of("likeCount", engagementService.unlike(postId, userId));
    }
}
