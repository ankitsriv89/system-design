package com.ankitsriv89.ticketbooking.domain;

import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "holds", indexes = {
    @Index(name = "idx_holds_seat_id", columnList = "seat_id"),
    @Index(name = "idx_holds_expires_at", columnList = "expires_at")
})
public class Hold {

    public enum Status { ACTIVE, EXPIRED, CONVERTED }

    @Id
    @GeneratedValue(strategy = GenerationType.UUID)
    private String id;

    @Column(name = "seat_id", nullable = false)
    private String seatId;

    @Column(name = "event_id", nullable = false)
    private String eventId;

    @Column(name = "user_id", nullable = false)
    private String userId;

    @Column(name = "expires_at", nullable = false)
    private Instant expiresAt;

    @Column(name = "created_at", nullable = false, updatable = false)
    private Instant createdAt = Instant.now();

    @Enumerated(EnumType.STRING)
    @Column(nullable = false)
    private Status status = Status.ACTIVE;

    public String getId() { return id; }
    public void setId(String id) { this.id = id; }
    public String getSeatId() { return seatId; }
    public void setSeatId(String seatId) { this.seatId = seatId; }
    public String getEventId() { return eventId; }
    public void setEventId(String eventId) { this.eventId = eventId; }
    public String getUserId() { return userId; }
    public void setUserId(String userId) { this.userId = userId; }
    public Instant getExpiresAt() { return expiresAt; }
    public void setExpiresAt(Instant expiresAt) { this.expiresAt = expiresAt; }
    public Instant getCreatedAt() { return createdAt; }
    public Status getStatus() { return status; }
    public void setStatus(Status status) { this.status = status; }
}
