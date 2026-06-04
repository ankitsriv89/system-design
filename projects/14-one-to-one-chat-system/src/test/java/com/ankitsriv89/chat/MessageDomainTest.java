package com.ankitsriv89.chat;

import com.ankitsriv89.chat.domain.Message;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class MessageDomainTest {

    @Test
    void newMessage_statusIsSent() {
        Message m = new Message(1L, 10L, "alice", "hello", 1L);
        assertEquals(Message.Status.SENT, m.getStatus());
    }

    @Test
    void markDelivered_transitionsFromSent() {
        Message m = new Message(1L, 10L, "alice", "hello", 1L);
        m.markDelivered();
        assertEquals(Message.Status.DELIVERED, m.getStatus());
    }

    @Test
    void markDelivered_idempotentIfAlreadyRead() {
        Message m = new Message(1L, 10L, "alice", "hello", 1L);
        m.markRead();
        m.markDelivered(); // Should not downgrade READ → DELIVERED
        assertEquals(Message.Status.READ, m.getStatus());
    }

    @Test
    void markRead_alwaysUpgrades() {
        Message m = new Message(1L, 10L, "alice", "hello", 1L);
        m.markDelivered();
        m.markRead();
        assertEquals(Message.Status.READ, m.getStatus());
    }

    @Test
    void idGenerator_monotonicallyIncreasing() {
        com.ankitsriv89.chat.service.IdGenerator gen = new com.ankitsriv89.chat.service.IdGenerator();
        long prev = gen.nextId();
        for (int i = 0; i < 100; i++) {
            long next = gen.nextId();
            assertTrue(next > prev, "ID must be strictly increasing");
            prev = next;
        }
    }
}
