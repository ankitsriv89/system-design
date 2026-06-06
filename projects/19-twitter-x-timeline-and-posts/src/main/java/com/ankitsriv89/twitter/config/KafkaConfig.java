package com.ankitsriv89.twitter.config;

import org.apache.kafka.clients.admin.NewTopic;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.kafka.config.TopicBuilder;

@Configuration
public class KafkaConfig {

    @Value("${twitter.kafka.topic.tweets}")
    private String tweetsTopic;

    // Partitioned by author_id so tweet.created events for the same author land
    // on one partition, preserving per-author ordering during fanout and indexing.
    @Bean
    public NewTopic tweetsTopic() {
        return TopicBuilder.name(tweetsTopic).partitions(6).replicas(1).build();
    }
}
