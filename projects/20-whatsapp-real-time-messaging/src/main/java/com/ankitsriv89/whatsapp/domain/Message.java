package com.ankitsriv89.whatsapp.domain;

import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "message")
public class Message {

    @Id @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "chat_id", nullable = false)
    private String chatId;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "sender_id")
    private AppUser sender;

    // Server stores opaque ciphertext; plaintext never leaves the client.
    @Column(nullable = false)
    private byte[] ciphertext;

    @Column(name = "created_at", nullable = false, updatable = false)
    private Instant createdAt = Instant.now();

    protected Message() {}

    public Message(String chatId, AppUser sender, byte[] ciphertext) {
        this.chatId = chatId;
        this.sender = sender;
        this.ciphertext = ciphertext;
    }

    public Long getId() { return id; }
    public String getChatId() { return chatId; }
    public AppUser getSender() { return sender; }
    public byte[] getCiphertext() { return ciphertext; }
    public Instant getCreatedAt() { return createdAt; }
}
