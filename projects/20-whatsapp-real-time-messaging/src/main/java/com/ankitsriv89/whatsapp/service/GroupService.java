package com.ankitsriv89.whatsapp.service;

import com.ankitsriv89.whatsapp.domain.*;
import com.ankitsriv89.whatsapp.dto.GroupCreateRequest;
import com.ankitsriv89.whatsapp.dto.GroupResponse;
import com.ankitsriv89.whatsapp.repository.*;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;

@Service
public class GroupService {

    private final ChatGroupRepository groups;
    private final GroupMemberRepository groupMembers;
    private final UserRepository users;

    public GroupService(ChatGroupRepository groups, GroupMemberRepository groupMembers, UserRepository users) {
        this.groups = groups;
        this.groupMembers = groupMembers;
        this.users = users;
    }

    @Transactional
    public GroupResponse create(String ownerUsername, GroupCreateRequest req) {
        AppUser owner = users.findByUsername(ownerUsername)
                .orElseThrow(() -> new IllegalStateException("Owner not found"));
        ChatGroup group = groups.save(new ChatGroup(req.name(), owner));
        groupMembers.save(new GroupMember(group, owner));
        if (req.memberUserIds() != null) {
            for (Long uid : req.memberUserIds()) {
                if (!uid.equals(owner.getId())) {
                    AppUser member = users.findById(uid)
                            .orElseThrow(() -> new IllegalArgumentException("User not found: " + uid));
                    groupMembers.save(new GroupMember(group, member));
                }
            }
        }
        return GroupResponse.from(group);
    }

    @Transactional
    public void addMember(Long groupId, Long userId, String requesterUsername) {
        ChatGroup group = groups.findById(groupId)
                .orElseThrow(() -> new IllegalArgumentException("Group not found"));
        if (!group.getOwner().getUsername().equals(requesterUsername)) {
            throw new SecurityException("Only the group owner can add members");
        }
        if (groupMembers.existsByGroupIdAndUserId(groupId, userId)) return;
        AppUser user = users.findById(userId)
                .orElseThrow(() -> new IllegalArgumentException("User not found: " + userId));
        groupMembers.save(new GroupMember(group, user));
    }

    public List<GroupResponse> listForUser(String username) {
        AppUser user = users.findByUsername(username)
                .orElseThrow(() -> new IllegalStateException("User not found"));
        return groups.findGroupsForUser(user.getId()).stream().map(GroupResponse::from).toList();
    }
}
