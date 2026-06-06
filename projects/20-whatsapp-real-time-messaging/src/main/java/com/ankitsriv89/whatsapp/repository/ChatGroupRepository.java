package com.ankitsriv89.whatsapp.repository;

import com.ankitsriv89.whatsapp.domain.ChatGroup;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import java.util.List;

public interface ChatGroupRepository extends JpaRepository<ChatGroup, Long> {

    @Query("SELECT g FROM ChatGroup g JOIN g.members m WHERE m.user.id = :userId")
    List<ChatGroup> findGroupsForUser(Long userId);
}
