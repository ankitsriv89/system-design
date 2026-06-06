package com.ankitsriv89.whatsapp.domain;

import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "app_user")
public class AppUser {

    @Id @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(nullable = false, unique = true)
    private String username;

    @Column(name = "created_at", nullable = false, updatable = false)
    private Instant createdAt = Instant.now();

    protected AppUser() {}

    public AppUser(String username) {
        this.username = username;
    }

    public Long getId() { return id; }
    public String getUsername() { return username; }
    public Instant getCreatedAt() { return createdAt; }
}
