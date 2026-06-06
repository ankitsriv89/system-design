package com.ankitsriv89.instagram.domain;

import jakarta.persistence.*;

import java.time.Instant;

/** A post: a user's media plus a caption. */
@Entity
@Table(name = "posts")
public class Post {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "user_id", nullable = false)
    private Long userId;

    @Column(name = "media_id", nullable = false)
    private Long mediaId;

    @Column(columnDefinition = "text")
    private String caption;

    @Column(name = "created_at", nullable = false)
    private Instant createdAt = Instant.now();

    protected Post() {
    }

    public Post(Long userId, Long mediaId, String caption) {
        this.userId = userId;
        this.mediaId = mediaId;
        this.caption = caption;
    }

    public Long getId() { return id; }
    public Long getUserId() { return userId; }
    public Long getMediaId() { return mediaId; }
    public String getCaption() { return caption; }
    public Instant getCreatedAt() { return createdAt; }
}
