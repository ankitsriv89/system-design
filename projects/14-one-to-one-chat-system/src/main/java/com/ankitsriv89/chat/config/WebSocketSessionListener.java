package com.ankitsriv89.chat.config;

import com.ankitsriv89.chat.service.PresenceService;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.context.event.EventListener;
import org.springframework.messaging.simp.stomp.StompHeaderAccessor;
import org.springframework.stereotype.Component;
import org.springframework.web.socket.messaging.SessionConnectedEvent;
import org.springframework.web.socket.messaging.SessionDisconnectEvent;

import java.security.Principal;

// Marks users online/offline based on STOMP session lifecycle events.
@Component
public class WebSocketSessionListener {

    private static final Logger log = LoggerFactory.getLogger(WebSocketSessionListener.class);

    private final PresenceService presence;

    public WebSocketSessionListener(PresenceService presence) {
        this.presence = presence;
    }

    @EventListener
    public void onConnect(SessionConnectedEvent event) {
        Principal principal = event.getUser();
        if (principal != null) {
            presence.heartbeat(principal.getName());
            log.debug("WS connected user={}", principal.getName());
        }
    }

    @EventListener
    public void onDisconnect(SessionDisconnectEvent event) {
        StompHeaderAccessor accessor = StompHeaderAccessor.wrap(event.getMessage());
        Principal principal = accessor.getUser();
        if (principal != null) {
            presence.markOffline(principal.getName());
            log.debug("WS disconnected user={}", principal.getName());
        }
    }
}
