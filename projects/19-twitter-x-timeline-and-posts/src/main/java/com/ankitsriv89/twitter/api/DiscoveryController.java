package com.ankitsriv89.twitter.api;

import com.ankitsriv89.twitter.dto.SearchHit;
import com.ankitsriv89.twitter.dto.Trend;
import com.ankitsriv89.twitter.service.SearchService;
import org.springframework.web.bind.annotation.*;

import java.util.List;

/**
 * Public discovery endpoints backed by OpenSearch. Both are open (no auth) since
 * search and trends operate over public tweets.
 */
@RestController
public class DiscoveryController {

    private final SearchService searchService;

    public DiscoveryController(SearchService searchService) {
        this.searchService = searchService;
    }

    @GetMapping("/v1/search")
    public List<SearchHit> search(@RequestParam("q") String query,
                                  @RequestParam(defaultValue = "20") int limit) {
        if (query == null || query.isBlank()) {
            throw new IllegalArgumentException("query parameter 'q' is required");
        }
        return searchService.search(query.trim(), limit);
    }

    @GetMapping("/v1/trends")
    public List<Trend> trends() {
        return searchService.trends();
    }
}
