package com.ankitsriv89.groupchat.metrics;

import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.MeterRegistry;
import org.springframework.stereotype.Component;

@Component
public class ChatMetrics {

    private final Counter messagesSent;
    private final Counter wsConnections;

    public ChatMetrics(MeterRegistry registry) {
        messagesSent = Counter.builder("groupchat_messages_sent_total")
                .description("Total messages sent across all groups")
                .register(registry);
        wsConnections = Counter.builder("groupchat_ws_connections_total")
                .description("Total WebSocket connections established")
                .register(registry);
    }

    public void recordMessage() { messagesSent.increment(); }
    public void recordConnection() { wsConnections.increment(); }
}
