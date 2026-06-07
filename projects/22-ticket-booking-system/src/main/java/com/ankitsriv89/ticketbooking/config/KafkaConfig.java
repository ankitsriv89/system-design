package com.ankitsriv89.ticketbooking.config;

import org.apache.kafka.clients.admin.NewTopic;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.kafka.config.TopicBuilder;

@Configuration
public class KafkaConfig {

    @Value("${ticket-booking.kafka.topic.holds:ticket.holds}")
    private String holdsTopic;

    @Value("${ticket-booking.kafka.topic.bookings:ticket.bookings}")
    private String bookingsTopic;

    @Bean
    public NewTopic holdsTopicBean() {
        return TopicBuilder.name(holdsTopic).partitions(3).replicas(1).build();
    }

    @Bean
    public NewTopic bookingsTopicBean() {
        return TopicBuilder.name(bookingsTopic).partitions(3).replicas(1).build();
    }
}
