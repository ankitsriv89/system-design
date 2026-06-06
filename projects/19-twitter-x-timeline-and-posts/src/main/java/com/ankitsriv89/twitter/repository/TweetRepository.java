package com.ankitsriv89.twitter.repository;

import com.ankitsriv89.twitter.domain.Tweet;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

import java.util.Collection;
import java.util.List;

public interface TweetRepository extends JpaRepository<Tweet, Long> {

    /**
     * Recent tweets by a set of authors, newest first. Used by the read path
     * (fanout-on-read) to pull celebrity tweets at query time, and by backfill
     * to reconstruct a timeline.
     */
    @Query("SELECT t FROM Tweet t WHERE t.authorId IN :authorIds AND t.deleted = false " +
           "ORDER BY t.createdAt DESC")
    List<Tweet> findRecentByAuthors(@Param("authorIds") Collection<String> authorIds, Pageable pageable);

    /** A single user's own tweets, newest first — the user (profile) timeline. */
    List<Tweet> findByAuthorIdAndDeletedFalseOrderByCreatedAtDesc(String authorId, Pageable pageable);
}
