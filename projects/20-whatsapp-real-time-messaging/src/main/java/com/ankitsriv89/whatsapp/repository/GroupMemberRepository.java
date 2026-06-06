package com.ankitsriv89.whatsapp.repository;

import com.ankitsriv89.whatsapp.domain.GroupMember;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import java.util.List;

public interface GroupMemberRepository extends JpaRepository<GroupMember, Long> {

    @Query("SELECT m FROM GroupMember m WHERE m.group.id = :groupId")
    List<GroupMember> findByGroupId(Long groupId);

    boolean existsByGroupIdAndUserId(Long groupId, Long userId);
}
