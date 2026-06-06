package com.ankitsriv89.instagram.repository;

import com.ankitsriv89.instagram.domain.Post;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;

import java.util.List;

public interface PostRepository extends JpaRepository<Post, Long> {

    /** Recent posts by a set of authors, newest first — the celebrity pull path. */
    List<Post> findByUserIdInOrderByCreatedAtDesc(List<Long> userIds, Pageable pageable);

    List<Post> findByUserIdOrderByCreatedAtDesc(Long userId, Pageable pageable);
}
