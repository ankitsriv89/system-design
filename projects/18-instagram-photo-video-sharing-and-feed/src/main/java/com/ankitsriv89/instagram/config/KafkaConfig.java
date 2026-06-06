package com.ankitsriv89.instagram.config;

import org.apache.kafka.clients.admin.NewTopic;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.kafka.config.TopicBuilder;

/**
 * Topic declarations. Media topics are keyed by mediaId; post topics by userId
 * so per-user ordering is preserved on a single partition during fanout.
 */
@Configuration
public class KafkaConfig {

    @Value("${instagram.kafka.topics.media-uploaded}")
    private String mediaUploaded;

    @Value("${instagram.kafka.topics.media-processed}")
    private String mediaProcessed;

    @Value("${instagram.kafka.topics.post-created}")
    private String postCreated;

    @Value("${instagram.kafka.topics.post-liked}")
    private String postLiked;

    @Bean
    public NewTopic mediaUploadedTopic() {
        return TopicBuilder.name(mediaUploaded).partitions(6).replicas(1).build();
    }

    @Bean
    public NewTopic mediaProcessedTopic() {
        return TopicBuilder.name(mediaProcessed).partitions(6).replicas(1).build();
    }

    @Bean
    public NewTopic postCreatedTopic() {
        return TopicBuilder.name(postCreated).partitions(6).replicas(1).build();
    }

    @Bean
    public NewTopic postLikedTopic() {
        return TopicBuilder.name(postLiked).partitions(6).replicas(1).build();
    }
}
