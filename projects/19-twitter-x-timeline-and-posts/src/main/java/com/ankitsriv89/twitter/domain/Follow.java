package com.ankitsriv89.twitter.domain;

import jakarta.persistence.*;

import java.time.Instant;

/**
 * Directed follow edge: {@code followerId} follows {@code followeeId}.
 * The social graph lives in PostgreSQL (durable, queryable for fanout targets).
 */
@Entity
@Table(name = "follows",
        uniqueConstraints = @UniqueConstraint(columnNames = {"follower_id", "followee_id"}))
public class Follow {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "follower_id", nullable = false)
    private String followerId;

    @Column(name = "followee_id", nullable = false)
    private String followeeId;

    @Column(name = "created_at", nullable = false)
    private Instant createdAt;

    protected Follow() {
    }

    public Follow(String followerId, String followeeId, Instant createdAt) {
        this.followerId = followerId;
        this.followeeId = followeeId;
        this.createdAt = createdAt;
    }

    public Long getId() { return id; }
    public String getFollowerId() { return followerId; }
    public String getFolloweeId() { return followeeId; }
    public Instant getCreatedAt() { return createdAt; }
}
