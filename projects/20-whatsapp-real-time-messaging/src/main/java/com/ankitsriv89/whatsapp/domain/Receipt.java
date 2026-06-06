package com.ankitsriv89.whatsapp.domain;

import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "receipt")
public class Receipt {

    @EmbeddedId
    private ReceiptId id;

    @ManyToOne(fetch = FetchType.LAZY)
    @MapsId("messageId")
    @JoinColumn(name = "message_id")
    private Message message;

    @ManyToOne(fetch = FetchType.LAZY)
    @MapsId("deviceId")
    @JoinColumn(name = "device_id")
    private Device device;

    @Column(nullable = false)
    private String state = ReceiptState.SENT.name();

    @Column(name = "updated_at", nullable = false)
    private Instant updatedAt = Instant.now();

    protected Receipt() {}

    public Receipt(Message message, Device device) {
        this.id = new ReceiptId(message.getId(), device.getId());
        this.message = message;
        this.device = device;
    }

    public ReceiptId getId() { return id; }
    public Message getMessage() { return message; }
    public Device getDevice() { return device; }
    public String getState() { return state; }
    public Instant getUpdatedAt() { return updatedAt; }

    public void advance(ReceiptState next) {
        this.state = next.name();
        this.updatedAt = Instant.now();
    }
}
