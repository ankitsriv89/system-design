package com.ankitsriv89.newsfeed.domain;

import jakarta.persistence.*;

import java.time.Instant;

/**
 * A post authored by a user. This is the durable source of truth — a post is
 * persisted here transactionally before any fanout event is published, so we
 * never fan out a post that wasn't durably stored ("post creation remains
 * durable before fanout").
 */
@Entity
@Table(name = "posts")
public class Post {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "author_id", nullable = false)
    private String authorId;

    @Column(nullable = false, length = 1000)
    private String body;

    @Column(name = "created_at", nullable = false)
    private Instant createdAt;

    @Column(nullable = false)
    private boolean deleted = false;

    protected Post() {
    }

    public Post(String authorId, String body, Instant createdAt) {
        this.authorId = authorId;
        this.body = body;
        this.createdAt = createdAt;
    }

    public Long getId() { return id; }
    public String getAuthorId() { return authorId; }
    public String getBody() { return body; }
    public Instant getCreatedAt() { return createdAt; }
    public boolean isDeleted() { return deleted; }
    public void setDeleted(boolean deleted) { this.deleted = deleted; }
}
