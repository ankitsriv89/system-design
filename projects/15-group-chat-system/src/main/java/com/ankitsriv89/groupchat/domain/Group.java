package com.ankitsriv89.groupchat.domain;

import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "groups")
public class Group {

    @Id
    private Long id;

    @Column(nullable = false, length = 128)
    private String name;

    @Column(name = "owner_id", nullable = false, length = 64)
    private String ownerId;

    @Column(name = "created_at", nullable = false, updatable = false)
    private Instant createdAt = Instant.now();

    public Group() {}

    public Group(Long id, String name, String ownerId) {
        this.id = id;
        this.name = name;
        this.ownerId = ownerId;
    }

    public Long getId() { return id; }
    public String getName() { return name; }
    public String getOwnerId() { return ownerId; }
    public Instant getCreatedAt() { return createdAt; }
    public void setName(String name) { this.name = name; }
}
