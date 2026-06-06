package com.ankitsriv89.whatsapp.domain;

import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "group_member",
       uniqueConstraints = @UniqueConstraint(columnNames = {"group_id", "user_id"}))
public class GroupMember {

    @Id @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "group_id")
    private ChatGroup group;

    @ManyToOne(fetch = FetchType.LAZY, optional = false)
    @JoinColumn(name = "user_id")
    private AppUser user;

    @Column(name = "joined_at", nullable = false, updatable = false)
    private Instant joinedAt = Instant.now();

    protected GroupMember() {}

    public GroupMember(ChatGroup group, AppUser user) {
        this.group = group;
        this.user = user;
    }

    public Long getId() { return id; }
    public ChatGroup getGroup() { return group; }
    public AppUser getUser() { return user; }
    public Instant getJoinedAt() { return joinedAt; }
}
