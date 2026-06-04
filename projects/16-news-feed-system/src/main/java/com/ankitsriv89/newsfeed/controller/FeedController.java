package com.ankitsriv89.newsfeed.controller;

import com.ankitsriv89.newsfeed.dto.FeedItemResponse;
import com.ankitsriv89.newsfeed.service.FeedService;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.web.bind.annotation.*;

import java.util.List;
import java.util.Map;

@RestController
@RequestMapping("/v1/feed")
public class FeedController {

    private final FeedService feedService;

    public FeedController(FeedService feedService) {
        this.feedService = feedService;
    }

    private static String currentUser() {
        return (String) SecurityContextHolder.getContext().getAuthentication().getPrincipal();
    }

    @GetMapping
    public List<FeedItemResponse> feed(@RequestParam(defaultValue = "20") int limit) {
        return feedService.homeFeed(currentUser(), limit);
    }

    /** Operator control: inspect/repair the materialized timeline for the caller. */
    @PostMapping("/backfill")
    public Map<String, Object> backfill(@RequestParam(defaultValue = "800") int max) {
        String user = currentUser();
        int n = feedService.backfill(user, max);
        return Map.of("user", user, "backfilledItems", n, "timelineSize", feedService.timelineSize(user));
    }

    @GetMapping("/stats")
    public Map<String, Object> stats() {
        String user = currentUser();
        return Map.of("user", user, "timelineSize", feedService.timelineSize(user));
    }
}
