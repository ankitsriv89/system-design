package com.ankitsriv89.ticketbooking.domain;

import jakarta.persistence.*;

@Entity
@Table(name = "seats", indexes = {
    @Index(name = "idx_seats_event_status", columnList = "event_id, status")
})
public class Seat {

    public enum Status { AVAILABLE, HELD, BOOKED }

    @Id
    @GeneratedValue(strategy = GenerationType.UUID)
    private String id;

    @Column(name = "event_id", nullable = false)
    private String eventId;

    @Column(name = "seat_number", nullable = false)
    private String seatNumber;

    @Column(name = "row_label")
    private String rowLabel;

    @Column(name = "section")
    private String section;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false)
    private Status status = Status.AVAILABLE;

    @Version
    private long version;

    public String getId() { return id; }
    public void setId(String id) { this.id = id; }
    public String getEventId() { return eventId; }
    public void setEventId(String eventId) { this.eventId = eventId; }
    public String getSeatNumber() { return seatNumber; }
    public void setSeatNumber(String seatNumber) { this.seatNumber = seatNumber; }
    public String getRowLabel() { return rowLabel; }
    public void setRowLabel(String rowLabel) { this.rowLabel = rowLabel; }
    public String getSection() { return section; }
    public void setSection(String section) { this.section = section; }
    public Status getStatus() { return status; }
    public void setStatus(Status status) { this.status = status; }
    public long getVersion() { return version; }
}
