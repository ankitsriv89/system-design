package com.ankitsriv89.proximityservice.config;

import org.apache.kafka.clients.admin.NewTopic;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.kafka.config.TopicBuilder;

@Configuration
public class KafkaConfig {

    @Value("${proximity.kafka.topics.location-updated:location.updated}")
    private String locationUpdatedTopic;

    @Value("${proximity.kafka.topics.nearby-queried:nearby.queried}")
    private String nearbyQueriedTopic;

    @Value("${proximity.kafka.topics.location-expired:location.expired}")
    private String locationExpiredTopic;

    @Bean
    public NewTopic locationUpdatedTopic() {
        return TopicBuilder.name(locationUpdatedTopic).partitions(6).replicas(1).build();
    }

    @Bean
    public NewTopic nearbyQueriedTopic() {
        return TopicBuilder.name(nearbyQueriedTopic).partitions(3).replicas(1).build();
    }

    @Bean
    public NewTopic locationExpiredTopic() {
        return TopicBuilder.name(locationExpiredTopic).partitions(3).replicas(1).build();
    }
}
