package com.ankitsriv89.groupchat.repository;

import com.ankitsriv89.groupchat.domain.Message;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;

import java.util.List;
import java.util.Optional;

public interface MessageRepository extends JpaRepository<Message, Long> {

    List<Message> findTop50ByGroupIdOrderBySeqDesc(Long groupId);

    List<Message> findByGroupIdAndSeqLessThanOrderBySeqDesc(Long groupId, Long beforeSeq);

    @Query("SELECT MAX(m.seq) FROM Message m WHERE m.groupId = :groupId")
    Optional<Long> findMaxSeqByGroupId(Long groupId);
}
