package com.ankitsriv89.groupchat.domain;

import java.io.Serializable;
import java.util.Objects;

public class GroupMemberId implements Serializable {
    private Long groupId;
    private String userId;

    public GroupMemberId() {}
    public GroupMemberId(Long groupId, String userId) {
        this.groupId = groupId;
        this.userId = userId;
    }

    @Override public boolean equals(Object o) {
        if (this == o) return true;
        if (!(o instanceof GroupMemberId that)) return false;
        return Objects.equals(groupId, that.groupId) && Objects.equals(userId, that.userId);
    }

    @Override public int hashCode() { return Objects.hash(groupId, userId); }
}
