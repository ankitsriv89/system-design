package com.ankitsriv89.dropbox.domain;

import jakarta.persistence.*;
import lombok.Data;
import java.time.Instant;

@Entity
@Table(name = "sync_cursor")
@Data
public class SyncCursor {
    @Id
    @Column(name = "device_id", length = 128)
    private String deviceId;

    @Column(name = "owner_id", nullable = false, length = 64)
    private String ownerId;

    @Column(name = "last_event_id", nullable = false)
    private long lastEventId = 0L;

    @Column(name = "updated_at", nullable = false)
    private Instant updatedAt = Instant.now();
}
