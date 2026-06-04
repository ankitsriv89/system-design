package com.ankitsriv89.chat.repository;

import com.ankitsriv89.chat.domain.Conversation;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

import java.util.List;
import java.util.Optional;

public interface ConversationRepository extends JpaRepository<Conversation, Long> {

    @Query("SELECT c FROM Conversation c WHERE c.userA = :u OR c.userB = :u ORDER BY c.createdAt DESC")
    List<Conversation> findByParticipant(@Param("u") String userId);

    @Query("SELECT c FROM Conversation c WHERE (c.userA = :a AND c.userB = :b) OR (c.userA = :b AND c.userB = :a)")
    Optional<Conversation> findByParticipants(@Param("a") String userA, @Param("b") String userB);
}
