package com.ankitsriv89.groupchat.config;

import com.ankitsriv89.groupchat.metrics.ChatMetrics;
import com.ankitsriv89.groupchat.service.PresenceService;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.context.event.EventListener;
import org.springframework.messaging.simp.stomp.StompHeaderAccessor;
import org.springframework.stereotype.Component;
import org.springframework.web.socket.messaging.SessionConnectedEvent;
import org.springframework.web.socket.messaging.SessionDisconnectEvent;

@Component
public class WebSocketSessionListener {

    private static final Logger log = LoggerFactory.getLogger(WebSocketSessionListener.class);

    private final PresenceService presenceService;
    private final ChatMetrics metrics;

    public WebSocketSessionListener(PresenceService presenceService, ChatMetrics metrics) {
        this.presenceService = presenceService;
        this.metrics = metrics;
    }

    @EventListener
    public void onConnect(SessionConnectedEvent event) {
        StompHeaderAccessor sha = StompHeaderAccessor.wrap(event.getMessage());
        String userId = sha.getUser() != null ? sha.getUser().getName() : null;
        if (userId != null) {
            presenceService.online(userId);
            metrics.recordConnection();
            log.debug("ws connected user={}", userId);
        }
    }

    @EventListener
    public void onDisconnect(SessionDisconnectEvent event) {
        StompHeaderAccessor sha = StompHeaderAccessor.wrap(event.getMessage());
        String userId = sha.getUser() != null ? sha.getUser().getName() : null;
        if (userId != null) {
            presenceService.offline(userId);
            log.debug("ws disconnected user={}", userId);
        }
    }
}
