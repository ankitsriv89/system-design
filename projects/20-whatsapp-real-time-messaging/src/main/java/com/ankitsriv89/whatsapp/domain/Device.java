package com.ankitsriv89.whatsapp.domain;

import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "device")
public class Device {

    @Id @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "user_id")
    private AppUser user;

    @Column(name = "public_key", nullable = false, length = 2048)
    private String publicKey;

    @Column
    private String label;

    @Column(name = "created_at", nullable = false, updatable = false)
    private Instant createdAt = Instant.now();

    @Column(name = "last_seen")
    private Instant lastSeen;

    protected Device() {}

    public Device(AppUser user, String publicKey, String label) {
        this.user = user;
        this.publicKey = publicKey;
        this.label = label;
    }

    public Long getId() { return id; }
    public AppUser getUser() { return user; }
    public String getPublicKey() { return publicKey; }
    public String getLabel() { return label; }
    public Instant getCreatedAt() { return createdAt; }
    public Instant getLastSeen() { return lastSeen; }

    public void touch() { this.lastSeen = Instant.now(); }
}
