package com.ankitsriv89.newsfeed.controller;

import com.ankitsriv89.newsfeed.dto.FollowRequest;
import com.ankitsriv89.newsfeed.service.FeedService;
import com.ankitsriv89.newsfeed.service.FollowService;
import jakarta.validation.Valid;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.web.bind.annotation.*;

import java.util.Map;

@RestController
@RequestMapping("/v1/follows")
public class FollowController {

    private final FollowService followService;
    private final FeedService feedService;
    private final int backfillMax;

    public FollowController(FollowService followService,
                            FeedService feedService,
                            @Value("${newsfeed.feed.max-cached-items}") int backfillMax) {
        this.followService = followService;
        this.feedService = feedService;
        this.backfillMax = backfillMax;
    }

    private static String currentUser() {
        return (String) SecurityContextHolder.getContext().getAuthentication().getPrincipal();
    }

    @PostMapping
    public ResponseEntity<Map<String, Object>> follow(@Valid @RequestBody FollowRequest req) {
        String follower = currentUser();
        followService.follow(follower, req.followeeId().trim());
        // Backfill the new follower's timeline so the followee's recent history
        // appears immediately rather than only from the next post onward.
        int backfilled = feedService.backfill(follower, backfillMax);
        return ResponseEntity.ok(Map.of(
                "follower", follower,
                "followee", req.followeeId().trim(),
                "backfilledItems", backfilled));
    }
}
