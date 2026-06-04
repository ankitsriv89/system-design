package com.ankitsriv89.groupchat.service;

import org.springframework.stereotype.Component;

import java.time.Instant;
import java.util.concurrent.atomic.AtomicInteger;

@Component
public class IdGenerator {

    private static final long EPOCH = 1700000000000L;
    private final AtomicInteger seq = new AtomicInteger(0);
    private final int nodeId;

    public IdGenerator() {
        this.nodeId = (int) (ProcessHandle.current().pid() % 1024);
    }

    public long next() {
        long ts = Instant.now().toEpochMilli() - EPOCH;
        int s = seq.incrementAndGet() & 0xFFF;
        return (ts << 22) | ((long) nodeId << 12) | s;
    }
}
