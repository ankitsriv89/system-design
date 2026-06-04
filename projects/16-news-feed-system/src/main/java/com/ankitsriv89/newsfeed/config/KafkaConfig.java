package com.ankitsriv89.newsfeed.config;

import org.apache.kafka.clients.admin.NewTopic;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.kafka.config.TopicBuilder;

@Configuration
public class KafkaConfig {

    @Value("${newsfeed.kafka.topic.events}")
    private String eventsTopic;

    // Partitioned by author_id so post.created events for the same author land on
    // one partition, preserving per-author ordering during fanout.
    @Bean
    public NewTopic eventsTopic() {
        return TopicBuilder.name(eventsTopic).partitions(6).replicas(1).build();
    }
}
