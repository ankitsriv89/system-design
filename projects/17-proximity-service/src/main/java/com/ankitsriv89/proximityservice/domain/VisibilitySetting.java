package com.ankitsriv89.proximityservice.domain;

import jakarta.persistence.*;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.time.Instant;

@Entity
@Table(name = "visibility_settings")
@Data
@NoArgsConstructor
public class VisibilitySetting {

    @Id
    @Column(name = "user_id")
    private String userId;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false)
    private VisibilityMode mode = VisibilityMode.FRIENDS;

    @Column(name = "updated_at", nullable = false)
    private Instant updatedAt = Instant.now();

    public VisibilitySetting(String userId, VisibilityMode mode) {
        this.userId = userId;
        this.mode = mode;
        this.updatedAt = Instant.now();
    }
}
