package com.ankitsriv89.twitter.config;

import org.apache.http.HttpHost;
import org.opensearch.client.RestClient;
import org.opensearch.client.RestHighLevelClient;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * Wires the OpenSearch high-level REST client at the bundled single-node
 * cluster. OpenSearch is the public-discovery store: full-text tweet search and
 * hashtag trend aggregation are both served from the {@code tweets} index, which
 * is an eventually-consistent projection built off the tweet.created stream.
 */
@Configuration
public class OpenSearchConfig {

    @Bean(destroyMethod = "close")
    public RestHighLevelClient openSearchClient(@Value("${twitter.search.host}") String host,
                                                @Value("${twitter.search.port}") int port) {
        return new RestHighLevelClient(
                RestClient.builder(new HttpHost(host, port, "http")));
    }
}
