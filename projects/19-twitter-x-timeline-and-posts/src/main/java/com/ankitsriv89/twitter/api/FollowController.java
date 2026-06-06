package com.ankitsriv89.twitter.api;

import com.ankitsriv89.twitter.dto.FollowRequest;
import com.ankitsriv89.twitter.service.FollowService;
import com.ankitsriv89.twitter.service.TimelineService;
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
    private final TimelineService timelineService;
    private final int backfillMax;

    public FollowController(FollowService followService,
                            TimelineService timelineService,
                            @Value("${twitter.timeline.max-cached-items}") int backfillMax) {
        this.followService = followService;
        this.timelineService = timelineService;
        this.backfillMax = backfillMax;
    }

    private static String currentUser() {
        return (String) SecurityContextHolder.getContext().getAuthentication().getPrincipal();
    }

    @PostMapping
    public ResponseEntity<Map<String, Object>> follow(@Valid @RequestBody FollowRequest req) {
        String follower = currentUser();
        followService.follow(follower, req.followeeId().trim());
        // Backfill the follower's timeline so the followee's recent history
        // appears immediately rather than only from the next tweet onward.
        int backfilled = timelineService.backfill(follower, backfillMax);
        return ResponseEntity.ok(Map.of(
                "follower", follower,
                "followee", req.followeeId().trim(),
                "backfilledItems", backfilled));
    }
}
