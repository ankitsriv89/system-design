package com.ankitsriv89.newsfeed.store;

import com.ankitsriv89.newsfeed.dto.PostCreatedEvent;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.stereotype.Component;

/**
 * Thin wrapper over KafkaTemplate for domain events. Keyed by authorId so a
 * single author's post.created events keep per-author order on one partition.
 */
@Component
public class EventPublisher {

    private final KafkaTemplate<String, Object> kafka;
    private final String eventsTopic;

    public EventPublisher(KafkaTemplate<String, Object> kafka,
                          @Value("${newsfeed.kafka.topic.events}") String eventsTopic) {
        this.kafka = kafka;
        this.eventsTopic = eventsTopic;
    }

    public void publishPostCreated(PostCreatedEvent event) {
        kafka.send(eventsTopic, event.authorId(), event);
    }
}
