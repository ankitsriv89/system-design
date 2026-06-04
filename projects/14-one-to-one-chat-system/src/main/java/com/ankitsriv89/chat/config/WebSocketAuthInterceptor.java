package com.ankitsriv89.chat.config;

import org.springframework.messaging.Message;
import org.springframework.messaging.MessageChannel;
import org.springframework.messaging.simp.stomp.StompCommand;
import org.springframework.messaging.simp.stomp.StompHeaderAccessor;
import org.springframework.messaging.support.ChannelInterceptor;
import org.springframework.messaging.support.MessageHeaderAccessor;
import org.springframework.security.authentication.UsernamePasswordAuthenticationToken;
import org.springframework.security.core.authority.SimpleGrantedAuthority;
import org.springframework.stereotype.Component;

import java.util.List;

// Authenticates STOMP CONNECT frames by reading the JWT from the
// "Authorization" native header, then sets the Principal on the session.
// This allows SimpMessagingTemplate.convertAndSendToUser() to route correctly.
@Component
public class WebSocketAuthInterceptor implements ChannelInterceptor {

    private final JwtUtil jwt;

    public WebSocketAuthInterceptor(JwtUtil jwt) {
        this.jwt = jwt;
    }

    @Override
    public Message<?> preSend(Message<?> message, MessageChannel channel) {
        StompHeaderAccessor accessor =
            MessageHeaderAccessor.getAccessor(message, StompHeaderAccessor.class);

        if (accessor != null && StompCommand.CONNECT.equals(accessor.getCommand())) {
            String authHeader = accessor.getFirstNativeHeader("Authorization");
            if (authHeader != null && authHeader.startsWith("Bearer ")) {
                try {
                    String userId = jwt.validate(authHeader.substring(7));
                    accessor.setUser(new UsernamePasswordAuthenticationToken(
                        userId, null, List.of(new SimpleGrantedAuthority("ROLE_USER"))
                    ));
                } catch (Exception ignored) {
                    // No principal set — STOMP session will have anonymous user
                }
            }
        }
        return message;
    }
}
