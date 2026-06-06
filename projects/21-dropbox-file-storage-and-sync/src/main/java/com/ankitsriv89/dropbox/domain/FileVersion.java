package com.ankitsriv89.dropbox.domain;

import jakarta.persistence.*;
import lombok.Data;
import java.time.Instant;
import java.util.UUID;

@Entity
@Table(name = "file_version")
@Data
public class FileVersion {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "file_id", nullable = false)
    private UUID fileId;

    @Column(nullable = false)
    private int version;

    @Column(name = "object_key", nullable = false, length = 1024)
    private String objectKey;

    @Column(nullable = false, length = 64)
    private String checksum;

    @Column(name = "size_bytes")
    private long sizeBytes;

    @Column(name = "created_at", nullable = false, updatable = false)
    private Instant createdAt = Instant.now();
}
