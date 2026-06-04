package com.ankitsriv89.groupchat.kafka;

import com.ankitsriv89.groupchat.controller.WebSocketHub;
import com.ankitsriv89.groupchat.dto.KafkaMessageEvent;
import com.ankitsriv89.groupchat.dto.WsEnvelope;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.stereotype.Component;

@Component
public class MessageConsumer {

    private static final Logger log = LoggerFactory.getLogger(MessageConsumer.class);

    private final WebSocketHub hub;

    public MessageConsumer(WebSocketHub hub) {
        this.hub = hub;
    }

    @KafkaListener(topics = "${groupchat.kafka.topic.messages}", groupId = "groupchat-service")
    public void onMessage(KafkaMessageEvent event) {
        log.debug("fanout group={} seq={}", event.getGroupId(), event.getSeq());
        WsEnvelope env = new WsEnvelope();
        env.setType("message");
        env.setGroupId(event.getGroupId());
        env.setSenderId(event.getSenderId());
        env.setBody(event.getBody());
        env.setSeq(event.getSeq());
        hub.fanout(event.getGroupId(), env);
    }
}
