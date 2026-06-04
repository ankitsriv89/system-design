package com.ankitsriv89.chat.domain;

import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "messages")
public class Message {

    public enum Status { SENT, DELIVERED, READ }

    @Id
    private Long id;

    @Column(name = "conversation_id", nullable = false)
    private Long conversationId;

    @Column(name = "sender_id", nullable = false, length = 64)
    private String senderId;

    @Column(name = "body", nullable = false)
    private String body;

    @Column(name = "seq", nullable = false)
    private long seq;

    @Enumerated(EnumType.STRING)
    @Column(name = "status", nullable = false, length = 16)
    private Status status = Status.SENT;

    @Column(name = "created_at", nullable = false)
    private Instant createdAt = Instant.now();

    protected Message() {}

    public Message(Long id, Long conversationId, String senderId, String body, long seq) {
        this.id = id;
        this.conversationId = conversationId;
        this.senderId = senderId;
        this.body = body;
        this.seq = seq;
        this.status = Status.SENT;
        this.createdAt = Instant.now();
    }

    public Long getId() { return id; }
    public Long getConversationId() { return conversationId; }
    public String getSenderId() { return senderId; }
    public String getBody() { return body; }
    public long getSeq() { return seq; }
    public Status getStatus() { return status; }
    public Instant getCreatedAt() { return createdAt; }

    public void markDelivered() {
        if (status == Status.SENT) status = Status.DELIVERED;
    }

    public void markRead() {
        status = Status.READ;
    }
}
