package com.ankitsriv89.twitter.domain;

import jakarta.persistence.*;

import java.time.Instant;

/**
 * A tweet authored by a user. This is the durable source of truth — a tweet is
 * persisted here transactionally before any tweet.created event is published,
 * so we never fan out or index a tweet that wasn't durably stored.
 *
 * <p>Soft delete: the row remains for audit, but the read path filters it out
 * and the timeline assembler drops it, so deletes are honored lazily at read
 * time even if a stale Redis timeline entry lingers.
 */
@Entity
@Table(name = "tweets")
public class Tweet {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "author_id", nullable = false)
    private String authorId;

    @Column(nullable = false, length = 280)
    private String text;

    @Column(name = "created_at", nullable = false)
    private Instant createdAt;

    @Column(nullable = false)
    private boolean deleted = false;

    protected Tweet() {
    }

    public Tweet(String authorId, String text, Instant createdAt) {
        this.authorId = authorId;
        this.text = text;
        this.createdAt = createdAt;
    }

    public Long getId() { return id; }
    public String getAuthorId() { return authorId; }
    public String getText() { return text; }
    public Instant getCreatedAt() { return createdAt; }
    public boolean isDeleted() { return deleted; }
    public void setDeleted(boolean deleted) { this.deleted = deleted; }
}
