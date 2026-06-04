package com.ankitsriv89.groupchat.repository;

import com.ankitsriv89.groupchat.domain.GroupMember;
import com.ankitsriv89.groupchat.domain.GroupMemberId;
import org.springframework.data.jpa.repository.JpaRepository;

import java.util.List;

public interface GroupMemberRepository extends JpaRepository<GroupMember, GroupMemberId> {
    List<GroupMember> findByGroupId(Long groupId);
    List<GroupMember> findByUserId(String userId);
    boolean existsByGroupIdAndUserId(Long groupId, String userId);
}
