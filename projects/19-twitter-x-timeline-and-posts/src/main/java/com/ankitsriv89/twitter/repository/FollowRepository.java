package com.ankitsriv89.twitter.repository;

import com.ankitsriv89.twitter.domain.Follow;
import org.springframework.data.jpa.repository.JpaRepository;

import java.util.List;

public interface FollowRepository extends JpaRepository<Follow, Long> {

    boolean existsByFollowerIdAndFolloweeId(String followerId, String followeeId);

    /** Who does this user follow? Drives the read-path pull and backfill. */
    List<Follow> findByFollowerId(String followerId);

    /** Who follows this user? These are the fanout-on-write targets. */
    List<Follow> findByFolloweeId(String followeeId);

    long countByFolloweeId(String followeeId);
}
