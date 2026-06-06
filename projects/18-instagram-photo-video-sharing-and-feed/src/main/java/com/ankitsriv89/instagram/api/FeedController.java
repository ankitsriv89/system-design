package com.ankitsriv89.instagram.api;

import com.ankitsriv89.instagram.dto.FeedItemResponse;
import com.ankitsriv89.instagram.service.FeedService;
import org.springframework.web.bind.annotation.*;

import java.util.List;

/** Home feed read endpoint. */
@RestController
@RequestMapping("/v1/feed")
public class FeedController {

    private final FeedService feedService;

    public FeedController(FeedService feedService) {
        this.feedService = feedService;
    }

    @GetMapping
    public List<FeedItemResponse> feed(
            @RequestHeader("X-User-Id") Long userId,
            @RequestParam(defaultValue = "0") int limit) {
        return feedService.feed(userId, limit);
    }
}
