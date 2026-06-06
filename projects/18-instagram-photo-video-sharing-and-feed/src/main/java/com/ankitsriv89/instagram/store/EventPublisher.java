package com.ankitsriv89.instagram.store;

import com.ankitsriv89.instagram.dto.MediaProcessedEvent;
import com.ankitsriv89.instagram.dto.MediaUploadedEvent;
import com.ankitsriv89.instagram.dto.PostCreatedEvent;
import com.ankitsriv89.instagram.dto.PostLikedEvent;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.stereotype.Component;

/**
 * Thin wrapper over KafkaTemplate for domain events. Media events are keyed by
 * mediaId so the variant worker processes a single media object in order.
 */
@Component
public class EventPublisher {

    private final KafkaTemplate<String, Object> kafka;
    private final String mediaUploadedTopic;
    private final String mediaProcessedTopic;
    private final String postCreatedTopic;
    private final String postLikedTopic;

    public EventPublisher(KafkaTemplate<String, Object> kafka,
                          @Value("${instagram.kafka.topics.media-uploaded}") String mediaUploadedTopic,
                          @Value("${instagram.kafka.topics.media-processed}") String mediaProcessedTopic,
                          @Value("${instagram.kafka.topics.post-created}") String postCreatedTopic,
                          @Value("${instagram.kafka.topics.post-liked}") String postLikedTopic) {
        this.kafka = kafka;
        this.mediaUploadedTopic = mediaUploadedTopic;
        this.mediaProcessedTopic = mediaProcessedTopic;
        this.postCreatedTopic = postCreatedTopic;
        this.postLikedTopic = postLikedTopic;
    }

    public void publishMediaUploaded(MediaUploadedEvent event) {
        kafka.send(mediaUploadedTopic, String.valueOf(event.mediaId()), event);
    }

    public void publishMediaProcessed(MediaProcessedEvent event) {
        kafka.send(mediaProcessedTopic, String.valueOf(event.mediaId()), event);
    }

    // Keyed by userId so a single author's post.created events keep per-author
    // order on one partition during fanout.
    public void publishPostCreated(PostCreatedEvent event) {
        kafka.send(postCreatedTopic, String.valueOf(event.userId()), event);
    }

    public void publishPostLiked(PostLikedEvent event) {
        kafka.send(postLikedTopic, String.valueOf(event.postId()), event);
    }
}
