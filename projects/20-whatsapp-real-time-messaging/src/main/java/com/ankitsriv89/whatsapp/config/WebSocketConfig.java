package com.ankitsriv89.whatsapp.config;

import com.ankitsriv89.whatsapp.api.SessionHandler;
import org.springframework.context.annotation.Configuration;
import org.springframework.web.socket.config.annotation.EnableWebSocket;
import org.springframework.web.socket.config.annotation.WebSocketConfigurer;
import org.springframework.web.socket.config.annotation.WebSocketHandlerRegistry;

@Configuration
@EnableWebSocket
public class WebSocketConfig implements WebSocketConfigurer {

    private final SessionHandler sessionHandler;

    public WebSocketConfig(SessionHandler sessionHandler) {
        this.sessionHandler = sessionHandler;
    }

    @Override
    public void registerWebSocketHandlers(WebSocketHandlerRegistry registry) {
        registry.addHandler(sessionHandler, "/ws/v1/session")
                .setAllowedOrigins("*");
    }
}
