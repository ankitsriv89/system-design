package com.ankitsriv89.whatsapp.domain;

import jakarta.persistence.*;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;

@Entity
@Table(name = "chat_group")
public class ChatGroup {

    @Id @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(nullable = false)
    private String name;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "owner_id")
    private AppUser owner;

    @Column(name = "created_at", nullable = false, updatable = false)
    private Instant createdAt = Instant.now();

    @OneToMany(mappedBy = "group", cascade = CascadeType.ALL, orphanRemoval = true)
    private List<GroupMember> members = new ArrayList<>();

    protected ChatGroup() {}

    public ChatGroup(String name, AppUser owner) {
        this.name = name;
        this.owner = owner;
    }

    public Long getId() { return id; }
    public String getName() { return name; }
    public AppUser getOwner() { return owner; }
    public Instant getCreatedAt() { return createdAt; }
    public List<GroupMember> getMembers() { return members; }

    public String chatId() { return "group:" + id; }
}
