package com.ankitsriv89.groupchat.domain;

import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "group_members")
@IdClass(GroupMemberId.class)
public class GroupMember {

    @Id
    @Column(name = "group_id")
    private Long groupId;

    @Id
    @Column(name = "user_id", length = 64)
    private String userId;

    @Column(name = "joined_at", nullable = false)
    private Instant joinedAt = Instant.now();

    @Column(nullable = false, length = 16)
    private String role = "MEMBER";

    public GroupMember() {}

    public GroupMember(Long groupId, String userId, String role) {
        this.groupId = groupId;
        this.userId = userId;
        this.role = role;
    }

    public Long getGroupId() { return groupId; }
    public String getUserId() { return userId; }
    public String getRole() { return role; }
    public Instant getJoinedAt() { return joinedAt; }
}
