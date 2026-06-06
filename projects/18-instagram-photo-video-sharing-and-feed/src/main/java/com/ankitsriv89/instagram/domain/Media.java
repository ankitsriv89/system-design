package com.ankitsriv89.instagram.domain;

import jakarta.persistence.*;
import org.hibernate.annotations.JdbcTypeCode;
import org.hibernate.type.SqlTypes;

import java.time.Instant;
import java.util.HashMap;
import java.util.Map;

/**
 * A media object (photo/video). Bytes live in object storage (MinIO/CDN); this
 * row is the durable metadata. {@code objectKey} is the original's key; once the
 * variant worker runs, {@code variants} maps a variant name (e.g. "thumbnail")
 * to its object key.
 */
@Entity
@Table(name = "media")
public class Media {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "owner_id", nullable = false)
    private Long ownerId;

    @Column(name = "object_key", nullable = false, length = 512)
    private String objectKey;

    @Column(name = "content_type", nullable = false, length = 128)
    private String contentType;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false, length = 16)
    private MediaStatus status = MediaStatus.PENDING;

    @JdbcTypeCode(SqlTypes.JSON)
    @Column(nullable = false, columnDefinition = "jsonb")
    private Map<String, String> variants = new HashMap<>();

    @Column(name = "created_at", nullable = false)
    private Instant createdAt = Instant.now();

    protected Media() {
    }

    public Media(Long ownerId, String objectKey, String contentType) {
        this.ownerId = ownerId;
        this.objectKey = objectKey;
        this.contentType = contentType;
    }

    public Long getId() { return id; }
    public Long getOwnerId() { return ownerId; }
    public String getObjectKey() { return objectKey; }
    public String getContentType() { return contentType; }
    public MediaStatus getStatus() { return status; }
    public void setStatus(MediaStatus status) { this.status = status; }
    public Map<String, String> getVariants() { return variants; }
    public void setVariants(Map<String, String> variants) { this.variants = variants; }
    public Instant getCreatedAt() { return createdAt; }
}
