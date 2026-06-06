package com.ankitsriv89.twitter.store;

import com.ankitsriv89.twitter.dto.SearchHit;
import com.ankitsriv89.twitter.dto.Trend;
import jakarta.annotation.PostConstruct;
import org.opensearch.action.index.IndexRequest;
import org.opensearch.action.search.SearchRequest;
import org.opensearch.action.search.SearchResponse;
import org.opensearch.client.RequestOptions;
import org.opensearch.client.RestHighLevelClient;
import org.opensearch.client.indices.CreateIndexRequest;
import org.opensearch.client.indices.GetIndexRequest;
import org.opensearch.common.xcontent.XContentType;
import org.opensearch.index.query.QueryBuilders;
import org.opensearch.search.aggregations.AggregationBuilders;
import org.opensearch.search.aggregations.bucket.terms.Terms;
import org.opensearch.search.builder.SearchSourceBuilder;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/**
 * OpenSearch adapter — the public discovery store. The {@code tweets} index is an
 * eventually-consistent projection of the tweet stream: the search indexer writes
 * each tweet here, then this store serves two read paths off it:
 *
 * <ul>
 *   <li><b>search</b> — full-text match over the {@code text} field;</li>
 *   <li><b>trends</b> — a terms aggregation over the {@code hashtags} keyword
 *       field, filtered to a recent time window, giving the top-N hashtags.</li>
 * </ul>
 *
 * Mapping note: {@code hashtags} is a keyword array extracted at index time so
 * the aggregation counts whole tags (not analyzed tokens). The index is created
 * lazily on startup with an explicit mapping.
 */
@Component
public class SearchStore {

    private static final Logger log = LoggerFactory.getLogger(SearchStore.class);

    private final RestHighLevelClient client;
    private final String index;
    private final int windowHours;
    private final int topN;

    public SearchStore(RestHighLevelClient client,
                       @Value("${twitter.search.index}") String index,
                       @Value("${twitter.trends.window-hours}") int windowHours,
                       @Value("${twitter.trends.top-n}") int topN) {
        this.client = client;
        this.index = index;
        this.windowHours = windowHours;
        this.topN = topN;
    }

    /**
     * Ensure the index exists with the right mapping. Best-effort on startup so
     * the app still boots if OpenSearch is briefly unreachable — the indexer
     * retries on the next event.
     */
    @PostConstruct
    public void ensureIndex() {
        try {
            boolean exists = client.indices()
                    .exists(new GetIndexRequest(index), RequestOptions.DEFAULT);
            if (exists) {
                return;
            }
            CreateIndexRequest req = new CreateIndexRequest(index);
            req.mapping("""
                {
                  "properties": {
                    "tweetId":   { "type": "long" },
                    "authorId":  { "type": "keyword" },
                    "text":      { "type": "text" },
                    "hashtags":  { "type": "keyword" },
                    "createdAt": { "type": "date" }
                  }
                }
                """, XContentType.JSON);
            client.indices().create(req, RequestOptions.DEFAULT);
            log.info("created OpenSearch index '{}'", index);
        } catch (Exception e) {
            log.warn("could not ensure OpenSearch index '{}' on startup: {}", index, e.getMessage());
        }
    }

    /** Index one tweet. Hashtags are extracted from the text and stored separately. */
    public void index(long tweetId, String authorId, String text, Instant createdAt) throws IOException {
        Map<String, Object> doc = Map.of(
                "tweetId", tweetId,
                "authorId", authorId,
                "text", text,
                "hashtags", extractHashtags(text),
                "createdAt", createdAt.toString());
        IndexRequest req = new IndexRequest(index)
                .id(Long.toString(tweetId))     // id = tweetId makes re-indexing idempotent
                .source(doc, XContentType.JSON);
        client.index(req, RequestOptions.DEFAULT);
    }

    /** Full-text search over tweet text, most-relevant first. */
    public List<SearchHit> search(String query, int limit) throws IOException {
        SearchSourceBuilder src = new SearchSourceBuilder()
                .query(QueryBuilders.matchQuery("text", query))
                .size(limit);
        SearchRequest req = new SearchRequest(index).source(src);
        SearchResponse resp = client.search(req, RequestOptions.DEFAULT);

        List<SearchHit> hits = new ArrayList<>();
        for (org.opensearch.search.SearchHit h : resp.getHits().getHits()) {
            Map<String, Object> m = h.getSourceAsMap();
            hits.add(new SearchHit(
                    ((Number) m.get("tweetId")).longValue(),
                    (String) m.get("authorId"),
                    (String) m.get("text"),
                    Instant.parse((String) m.get("createdAt")),
                    h.getScore()));
        }
        return hits;
    }

    /**
     * Top-N trending hashtags over the configured sliding window. A
     * {@code range} filter on {@code createdAt} restricts to recent tweets;
     * a {@code terms} aggregation on the {@code hashtags} keyword field counts
     * each tag. size=0 because we want only the aggregation, not the documents.
     */
    public List<Trend> trends() throws IOException {
        Instant since = Instant.now().minusSeconds((long) windowHours * 3600);
        SearchSourceBuilder src = new SearchSourceBuilder()
                .size(0)
                .query(QueryBuilders.rangeQuery("createdAt").gte(since.toString()))
                .aggregation(AggregationBuilders.terms("trending")
                        .field("hashtags")
                        .size(topN));
        SearchRequest req = new SearchRequest(index).source(src);
        SearchResponse resp = client.search(req, RequestOptions.DEFAULT);

        List<Trend> trends = new ArrayList<>();
        Terms agg = resp.getAggregations() == null ? null : resp.getAggregations().get("trending");
        if (agg != null) {
            for (Terms.Bucket b : agg.getBuckets()) {
                trends.add(new Trend(b.getKeyAsString(), b.getDocCount()));
            }
        }
        return trends;
    }

    /** Extract {@code #hashtag} tokens (lower-cased) from tweet text. */
    public static List<String> extractHashtags(String text) {
        List<String> tags = new ArrayList<>();
        var matcher = java.util.regex.Pattern
                .compile("#(\\w+)").matcher(text);
        while (matcher.find()) {
            tags.add(matcher.group(1).toLowerCase());
        }
        return tags;
    }
}
