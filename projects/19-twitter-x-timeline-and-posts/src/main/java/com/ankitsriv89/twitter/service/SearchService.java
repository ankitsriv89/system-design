package com.ankitsriv89.twitter.service;

import com.ankitsriv89.twitter.dto.SearchHit;
import com.ankitsriv89.twitter.dto.Trend;
import com.ankitsriv89.twitter.metrics.TwitterMetrics;
import com.ankitsriv89.twitter.store.SearchStore;
import org.springframework.stereotype.Service;

import java.io.IOException;
import java.util.List;

/**
 * Read-side facade over the OpenSearch index for the two public-discovery
 * endpoints: full-text search and trending hashtags. Both are eventually
 * consistent — a tweet appears here only after the search indexer has processed
 * its tweet.created event.
 */
@Service
public class SearchService {

    private final SearchStore search;
    private final TwitterMetrics metrics;

    public SearchService(SearchStore search, TwitterMetrics metrics) {
        this.search = search;
        this.metrics = metrics;
    }

    public List<SearchHit> search(String query, int limit) {
        metrics.recordSearchQuery();
        try {
            return search.search(query, limit > 0 ? limit : 20);
        } catch (IOException e) {
            throw new IllegalStateException("search backend unavailable: " + e.getMessage(), e);
        }
    }

    public List<Trend> trends() {
        metrics.recordTrendQuery();
        try {
            return search.trends();
        } catch (IOException e) {
            throw new IllegalStateException("search backend unavailable: " + e.getMessage(), e);
        }
    }
}
