package com.ankitsriv89.instagram.domain;

import jakarta.persistence.*;

import java.time.Instant;

/**
 * Minimal user. Identity for this project is a numeric id passed via the
 * {@code X-User-Id} header (seed-users model) rather than full JWT auth — the
 * milestone focus is the media pipeline and feed, not authentication.
 */
@Entity
@Table(name = "users")
public class User {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(nullable = false, unique = true, length = 64)
    private String username;

    @Column(name = "created_at", nullable = false)
    private Instant createdAt = Instant.now();

    protected User() {
    }

    public User(String username) {
        this.username = username;
    }

    public Long getId() { return id; }
    public String getUsername() { return username; }
    public Instant getCreatedAt() { return createdAt; }
}
