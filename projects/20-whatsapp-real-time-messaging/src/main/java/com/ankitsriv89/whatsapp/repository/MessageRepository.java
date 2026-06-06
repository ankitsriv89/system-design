package com.ankitsriv89.whatsapp.repository;

import com.ankitsriv89.whatsapp.domain.Message;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;
import java.time.Instant;
import java.util.List;

public interface MessageRepository extends JpaRepository<Message, Long> {
    List<Message> findByChatIdAndCreatedAtAfterOrderByCreatedAtAsc(
            String chatId, Instant after, Pageable pageable);

    List<Message> findByChatIdOrderByCreatedAtAsc(String chatId, Pageable pageable);
}
