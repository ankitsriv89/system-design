package com.ankitsriv89.newsfeed.repository;

import com.ankitsriv89.newsfeed.domain.Post;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.repository.query.Param;
import org.springframework.data.jpa.repository.Query;

import java.util.Collection;
import java.util.List;

public interface PostRepository extends JpaRepository<Post, Long> {

    /**
     * Recent posts by a set of authors, newest first. Used by the read-path
     * (fanout-on-read) to pull celebrity posts at query time, and by backfill
     * to reconstruct a timeline.
     */
    @Query("SELECT p FROM Post p WHERE p.authorId IN :authorIds AND p.deleted = false " +
           "ORDER BY p.createdAt DESC")
    List<Post> findRecentByAuthors(@Param("authorIds") Collection<String> authorIds, Pageable pageable);

    List<Post> findByAuthorIdAndDeletedFalseOrderByCreatedAtDesc(String authorId, Pageable pageable);
}
