package com.ankitsriv89.chat.metrics;

import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.MeterRegistry;
import org.springframework.stereotype.Component;

@Component
public class ChatMetrics {

    private final Counter messagesSent;
    private final Counter messagesDelivered;
    private final Counter messagesRead;

    public ChatMetrics(MeterRegistry registry) {
        messagesSent      = registry.counter("chat_messages_sent_total");
        messagesDelivered = registry.counter("chat_messages_delivered_total");
        messagesRead      = registry.counter("chat_messages_read_total");
    }

    public void recordSent()      { messagesSent.increment(); }
    public void recordDelivered() { messagesDelivered.increment(); }
    public void recordRead()      { messagesRead.increment(); }
}
