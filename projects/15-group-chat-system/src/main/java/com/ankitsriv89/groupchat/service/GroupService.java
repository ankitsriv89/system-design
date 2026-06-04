package com.ankitsriv89.groupchat.service;

import com.ankitsriv89.groupchat.domain.Group;
import com.ankitsriv89.groupchat.domain.GroupMember;
import com.ankitsriv89.groupchat.repository.GroupMemberRepository;
import com.ankitsriv89.groupchat.repository.GroupRepository;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;

@Service
public class GroupService {

    private final GroupRepository groupRepo;
    private final GroupMemberRepository memberRepo;
    private final IdGenerator ids;

    public GroupService(GroupRepository groupRepo, GroupMemberRepository memberRepo, IdGenerator ids) {
        this.groupRepo = groupRepo;
        this.memberRepo = memberRepo;
        this.ids = ids;
    }

    @Transactional
    public Group create(String name, String ownerId) {
        if (name == null || name.isBlank()) throw new IllegalArgumentException("name required");
        Group group = new Group(ids.next(), name.trim(), ownerId);
        groupRepo.save(group);
        memberRepo.save(new GroupMember(group.getId(), ownerId, "OWNER"));
        return group;
    }

    public List<Group> groupsForUser(String userId) {
        return memberRepo.findByUserId(userId).stream()
                .map(m -> groupRepo.findById(m.getGroupId()).orElseThrow())
                .toList();
    }

    public Group get(Long groupId) {
        return groupRepo.findById(groupId)
                .orElseThrow(() -> new IllegalArgumentException("group not found: " + groupId));
    }

    @Transactional
    public void addMember(Long groupId, String requesterId, String userId) {
        requireMember(groupId, requesterId);
        if (!memberRepo.existsByGroupIdAndUserId(groupId, userId)) {
            memberRepo.save(new GroupMember(groupId, userId, "MEMBER"));
        }
    }

    @Transactional
    public void removeMember(Long groupId, String requesterId, String userId) {
        Group group = get(groupId);
        if (!group.getOwnerId().equals(requesterId) && !requesterId.equals(userId)) {
            throw new SecurityException("only owner can remove others");
        }
        memberRepo.deleteById(new com.ankitsriv89.groupchat.domain.GroupMemberId(groupId, userId));
    }

    public List<GroupMember> members(Long groupId) {
        return memberRepo.findByGroupId(groupId);
    }

    public void requireMember(Long groupId, String userId) {
        if (!memberRepo.existsByGroupIdAndUserId(groupId, userId)) {
            throw new SecurityException("not a member of group " + groupId);
        }
    }
}
