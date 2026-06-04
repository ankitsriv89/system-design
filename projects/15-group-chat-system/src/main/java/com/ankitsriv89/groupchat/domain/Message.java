package com.ankitsriv89.groupchat.domain;

import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "messages")
public class Message {

    @Id
    private Long id;

    @Column(name = "group_id", nullable = false)
    private Long groupId;

    @Column(name = "sender_id", nullable = false, length = 64)
    private String senderId;

    @Column(nullable = false, columnDefinition = "TEXT")
    private String body;

    @Column(nullable = false)
    private Long seq;

    @Column(name = "created_at", nullable = false, updatable = false)
    private Instant createdAt = Instant.now();

    public Message() {}

    public Message(Long id, Long groupId, String senderId, String body, Long seq) {
        this.id = id;
        this.groupId = groupId;
        this.senderId = senderId;
        this.body = body;
        this.seq = seq;
    }

    public Long getId() { return id; }
    public Long getGroupId() { return groupId; }
    public String getSenderId() { return senderId; }
    public String getBody() { return body; }
    public Long getSeq() { return seq; }
    public Instant getCreatedAt() { return createdAt; }
}
