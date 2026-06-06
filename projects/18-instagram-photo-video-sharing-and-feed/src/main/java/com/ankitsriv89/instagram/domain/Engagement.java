package com.ankitsriv89.instagram.domain;

import jakarta.persistence.*;

import java.io.Serializable;
import java.time.Instant;
import java.util.Objects;

/** A like (or other engagement type) by a user on a post. */
@Entity
@Table(name = "engagements")
@IdClass(Engagement.EngagementId.class)
public class Engagement {

    @Id
    @Column(name = "post_id")
    private Long postId;

    @Id
    @Column(name = "user_id")
    private Long userId;

    @Id
    @Column(length = 16)
    private String type = "LIKE";

    @Column(name = "created_at", nullable = false)
    private Instant createdAt = Instant.now();

    protected Engagement() {
    }

    public Engagement(Long postId, Long userId, String type) {
        this.postId = postId;
        this.userId = userId;
        this.type = type;
    }

    public Long getPostId() { return postId; }
    public Long getUserId() { return userId; }
    public String getType() { return type; }
    public Instant getCreatedAt() { return createdAt; }

    public static class EngagementId implements Serializable {
        private Long postId;
        private Long userId;
        private String type;

        public EngagementId() {
        }

        public EngagementId(Long postId, Long userId, String type) {
            this.postId = postId;
            this.userId = userId;
            this.type = type;
        }

        @Override
        public boolean equals(Object o) {
            if (this == o) return true;
            if (!(o instanceof EngagementId that)) return false;
            return Objects.equals(postId, that.postId)
                    && Objects.equals(userId, that.userId)
                    && Objects.equals(type, that.type);
        }

        @Override
        public int hashCode() {
            return Objects.hash(postId, userId, type);
        }
    }
}
