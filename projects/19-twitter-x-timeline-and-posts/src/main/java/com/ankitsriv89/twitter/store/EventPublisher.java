package com.ankitsriv89.twitter.store;

import com.ankitsriv89.twitter.dto.TweetCreatedEvent;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.stereotype.Component;

/**
 * Thin wrapper over KafkaTemplate for domain events. Keyed by authorId so a
 * single author's tweet.created events keep per-author order on one partition.
 */
@Component
public class EventPublisher {

    private final KafkaTemplate<String, Object> kafka;
    private final String tweetsTopic;

    public EventPublisher(KafkaTemplate<String, Object> kafka,
                          @Value("${twitter.kafka.topic.tweets}") String tweetsTopic) {
        this.kafka = kafka;
        this.tweetsTopic = tweetsTopic;
    }

    public void publishTweetCreated(TweetCreatedEvent event) {
        kafka.send(tweetsTopic, event.authorId(), event);
    }
}
