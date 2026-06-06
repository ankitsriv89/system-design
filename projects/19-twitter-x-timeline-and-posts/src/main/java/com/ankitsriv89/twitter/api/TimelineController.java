package com.ankitsriv89.twitter.api;

import com.ankitsriv89.twitter.dto.TimelineItemResponse;
import com.ankitsriv89.twitter.service.TimelineService;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.web.bind.annotation.*;

import java.util.List;
import java.util.Map;

@RestController
public class TimelineController {

    private final TimelineService timelineService;

    public TimelineController(TimelineService timelineService) {
        this.timelineService = timelineService;
    }

    private static String currentUser() {
        return (String) SecurityContextHolder.getContext().getAuthentication().getPrincipal();
    }

    /** Home timeline for the caller — the hybrid-fanout merged read. */
    @GetMapping("/v1/home")
    public List<TimelineItemResponse> home(@RequestParam(defaultValue = "20") int limit) {
        return timelineService.homeTimeline(currentUser(), limit);
    }

    /** Operator control: rebuild/repair the materialized timeline for the caller. */
    @PostMapping("/v1/home/backfill")
    public Map<String, Object> backfill(@RequestParam(defaultValue = "800") int max) {
        String user = currentUser();
        int n = timelineService.backfill(user, max);
        return Map.of("user", user, "backfilledItems", n, "timelineSize", timelineService.timelineSize(user));
    }

    @GetMapping("/v1/home/stats")
    public Map<String, Object> stats() {
        String user = currentUser();
        return Map.of("user", user, "timelineSize", timelineService.timelineSize(user));
    }
}
