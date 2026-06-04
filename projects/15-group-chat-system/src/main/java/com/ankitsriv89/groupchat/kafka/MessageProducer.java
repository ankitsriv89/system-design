package com.ankitsriv89.groupchat.kafka;

import com.ankitsriv89.groupchat.dto.KafkaMessageEvent;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.stereotype.Component;

@Component
public class MessageProducer {

    private final KafkaTemplate<String, KafkaMessageEvent> kafka;
    private final String topic;

    public MessageProducer(KafkaTemplate<String, KafkaMessageEvent> kafka,
                           @Value("${groupchat.kafka.topic.messages}") String topic) {
        this.kafka = kafka;
        this.topic = topic;
    }

    public void publish(KafkaMessageEvent event) {
        kafka.send(topic, String.valueOf(event.getGroupId()), event);
    }
}
