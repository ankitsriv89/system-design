package com.ankitsriv89.groupchat.controller;

import com.ankitsriv89.groupchat.dto.WsEnvelope;
import org.springframework.messaging.simp.SimpMessagingTemplate;
import org.springframework.stereotype.Component;

@Component
public class WebSocketHub {

    private final SimpMessagingTemplate broker;

    public WebSocketHub(SimpMessagingTemplate broker) {
        this.broker = broker;
    }

    public void fanout(Long groupId, WsEnvelope envelope) {
        broker.convertAndSend("/topic/group/" + groupId, envelope);
    }
}
