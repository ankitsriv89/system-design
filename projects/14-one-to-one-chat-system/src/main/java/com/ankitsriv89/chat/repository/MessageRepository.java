package com.ankitsriv89.chat.repository;

import com.ankitsriv89.chat.domain.Message;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

import java.util.List;

public interface MessageRepository extends JpaRepository<Message, Long> {

    // Cursor-based pagination: fetch messages before a given seq (or all if beforeSeq is null)
    @Query("""
        SELECT m FROM Message m
        WHERE m.conversationId = :convId
          AND (:beforeSeq IS NULL OR m.seq < :beforeSeq)
        ORDER BY m.seq DESC
        LIMIT :limit
        """)
    List<Message> findPage(
        @Param("convId") Long conversationId,
        @Param("beforeSeq") Long beforeSeq,
        @Param("limit") int limit
    );
}
