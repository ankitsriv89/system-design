package com.ankitsriv89.groupchat.config;

import com.ankitsriv89.groupchat.service.GroupService;
import org.springframework.messaging.Message;
import org.springframework.messaging.MessageChannel;
import org.springframework.messaging.simp.stomp.StompCommand;
import org.springframework.messaging.simp.stomp.StompHeaderAccessor;
import org.springframework.messaging.support.ChannelInterceptor;
import org.springframework.messaging.support.MessageHeaderAccessor;
import org.springframework.security.authentication.UsernamePasswordAuthenticationToken;
import org.springframework.stereotype.Component;

import java.security.Principal;
import java.util.List;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

@Component
public class WebSocketAuthInterceptor implements ChannelInterceptor {

    private static final Pattern GROUP_TOPIC = Pattern.compile("^/topic/group/(\\d+)$");

    private final JwtUtil jwtUtil;
    private final GroupService groupService;

    public WebSocketAuthInterceptor(JwtUtil jwtUtil, GroupService groupService) {
        this.jwtUtil = jwtUtil;
        this.groupService = groupService;
    }

    @Override
    public Message<?> preSend(Message<?> message, MessageChannel channel) {
        StompHeaderAccessor sha = MessageHeaderAccessor.getAccessor(message, StompHeaderAccessor.class);
        if (sha == null) return message;

        if (StompCommand.CONNECT.equals(sha.getCommand())) {
            String token = extractToken(sha);
            if (token == null || !jwtUtil.isValid(token)) {
                throw new SecurityException("invalid or missing JWT on CONNECT");
            }
            String userId = jwtUtil.extractUserId(token);
            Principal principal = new UsernamePasswordAuthenticationToken(userId, null, List.of());
            sha.setUser(principal);
        }

        if (StompCommand.SUBSCRIBE.equals(sha.getCommand())) {
            String dest = sha.getDestination();
            if (dest != null) {
                Matcher m = GROUP_TOPIC.matcher(dest);
                if (m.matches()) {
                    Principal principal = sha.getUser();
                    if (principal == null) throw new SecurityException("not authenticated");
                    long groupId = Long.parseLong(m.group(1));
                    groupService.requireMember(groupId, principal.getName());
                }
            }
        }

        return message;
    }

    private String extractToken(StompHeaderAccessor sha) {
        // Check Authorization header first, then native STOMP header
        String auth = sha.getFirstNativeHeader("Authorization");
        if (auth != null && auth.startsWith("Bearer ")) return auth.substring(7);
        return null;
    }
}
