package com.ankitsriv89.instagram.domain;

import jakarta.persistence.*;

import java.time.Instant;

/** Directed follow edge: {@code followerId} follows {@code followeeId}. */
@Entity
@Table(name = "follows")
@IdClass(Follow.FollowId.class)
public class Follow {

    @Id
    @Column(name = "follower_id")
    private Long followerId;

    @Id
    @Column(name = "followee_id")
    private Long followeeId;

    @Column(name = "created_at", nullable = false)
    private Instant createdAt = Instant.now();

    protected Follow() {
    }

    public Follow(Long followerId, Long followeeId) {
        this.followerId = followerId;
        this.followeeId = followeeId;
    }

    public Long getFollowerId() { return followerId; }
    public Long getFolloweeId() { return followeeId; }
    public Instant getCreatedAt() { return createdAt; }

    /** Composite key (follower_id, followee_id). */
    public static class FollowId implements java.io.Serializable {
        private Long followerId;
        private Long followeeId;

        public FollowId() {
        }

        public FollowId(Long followerId, Long followeeId) {
            this.followerId = followerId;
            this.followeeId = followeeId;
        }

        @Override
        public boolean equals(Object o) {
            if (this == o) return true;
            if (!(o instanceof FollowId that)) return false;
            return java.util.Objects.equals(followerId, that.followerId)
                    && java.util.Objects.equals(followeeId, that.followeeId);
        }

        @Override
        public int hashCode() {
            return java.util.Objects.hash(followerId, followeeId);
        }
    }
}
