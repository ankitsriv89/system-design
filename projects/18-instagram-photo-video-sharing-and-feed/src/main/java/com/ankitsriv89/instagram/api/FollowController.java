package com.ankitsriv89.instagram.api;

import com.ankitsriv89.instagram.service.FollowService;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

/** Social-graph endpoints. {@code X-User-Id} is the trusted demo identity. */
@RestController
@RequestMapping("/v1/follows")
public class FollowController {

    private final FollowService followService;

    public FollowController(FollowService followService) {
        this.followService = followService;
    }

    @PutMapping("/{followeeId}")
    public ResponseEntity<Void> follow(
            @RequestHeader("X-User-Id") Long userId,
            @PathVariable Long followeeId) {
        followService.follow(userId, followeeId);
        return ResponseEntity.status(HttpStatus.NO_CONTENT).build();
    }

    @DeleteMapping("/{followeeId}")
    public ResponseEntity<Void> unfollow(
            @RequestHeader("X-User-Id") Long userId,
            @PathVariable Long followeeId) {
        followService.unfollow(userId, followeeId);
        return ResponseEntity.status(HttpStatus.NO_CONTENT).build();
    }
}
