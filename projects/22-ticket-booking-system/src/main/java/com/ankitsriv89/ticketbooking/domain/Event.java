package com.ankitsriv89.ticketbooking.domain;

import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "events")
public class Event {

    @Id
    @GeneratedValue(strategy = GenerationType.UUID)
    private String id;

    @Column(nullable = false)
    private String name;

    @Column(nullable = false)
    private String venue;

    @Column(name = "event_time", nullable = false)
    private Instant eventTime;

    @Column(name = "total_seats", nullable = false)
    private int totalSeats;

    @Column(name = "created_at", nullable = false, updatable = false)
    private Instant createdAt = Instant.now();

    public String getId() { return id; }
    public void setId(String id) { this.id = id; }
    public String getName() { return name; }
    public void setName(String name) { this.name = name; }
    public String getVenue() { return venue; }
    public void setVenue(String venue) { this.venue = venue; }
    public Instant getEventTime() { return eventTime; }
    public void setEventTime(Instant eventTime) { this.eventTime = eventTime; }
    public int getTotalSeats() { return totalSeats; }
    public void setTotalSeats(int totalSeats) { this.totalSeats = totalSeats; }
    public Instant getCreatedAt() { return createdAt; }
}
