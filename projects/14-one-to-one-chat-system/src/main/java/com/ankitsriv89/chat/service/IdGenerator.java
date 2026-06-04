package com.ankitsriv89.chat.service;

import org.springframework.stereotype.Component;

import java.util.concurrent.atomic.AtomicLong;

// Simple time-based ID generator sufficient for a single-node demo.
// In production this would be replaced by a distributed Snowflake service.
@Component
public class IdGenerator {

    private static final long EPOCH = 1_700_000_000_000L;
    private final AtomicLong sequence = new AtomicLong(0);

    public long nextId() {
        long ts = (System.currentTimeMillis() - EPOCH) << 12;
        long seq = sequence.incrementAndGet() & 0xFFFL;
        return ts | seq;
    }
}
