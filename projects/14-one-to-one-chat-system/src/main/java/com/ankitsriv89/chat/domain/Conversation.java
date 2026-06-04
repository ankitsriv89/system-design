package com.ankitsriv89.chat.domain;

import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "conversations")
public class Conversation {

    @Id
    private Long id;

    @Column(name = "user_a", nullable = false, length = 64)
    private String userA;

    @Column(name = "user_b", nullable = false, length = 64)
    private String userB;

    @Column(name = "created_at", nullable = false)
    private Instant createdAt = Instant.now();

    @Column(name = "last_seq", nullable = false)
    private long lastSeq = 0;

    protected Conversation() {}

    public Conversation(Long id, String userA, String userB) {
        // canonical order: userA < userB lexicographically
        if (userA.compareTo(userB) <= 0) {
            this.userA = userA;
            this.userB = userB;
        } else {
            this.userA = userB;
            this.userB = userA;
        }
        this.id = id;
        this.createdAt = Instant.now();
    }

    public Long getId() { return id; }
    public String getUserA() { return userA; }
    public String getUserB() { return userB; }
    public Instant getCreatedAt() { return createdAt; }
    public long getLastSeq() { return lastSeq; }

    public long incrementSeq() {
        return ++lastSeq;
    }

    public boolean hasParticipant(String userId) {
        return userA.equals(userId) || userB.equals(userId);
    }

    public String otherParticipant(String userId) {
        return userA.equals(userId) ? userB : userA;
    }
}
