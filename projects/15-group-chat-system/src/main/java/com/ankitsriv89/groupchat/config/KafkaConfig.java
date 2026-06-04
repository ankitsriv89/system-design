package com.ankitsriv89.groupchat.config;

import org.apache.kafka.clients.admin.NewTopic;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.kafka.config.TopicBuilder;

@Configuration
public class KafkaConfig {

    @Value("${groupchat.kafka.topic.messages}")
    private String messagesTopic;

    @Value("${groupchat.kafka.topic.events}")
    private String eventsTopic;

    @Bean
    public NewTopic messagesTopic() {
        return TopicBuilder.name(messagesTopic).partitions(6).replicas(1).build();
    }

    @Bean
    public NewTopic eventsTopic() {
        return TopicBuilder.name(eventsTopic).partitions(3).replicas(1).build();
    }
}
