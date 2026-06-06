package com.ankitsriv89.instagram.repository;

import com.ankitsriv89.instagram.domain.Follow;
import org.springframework.data.jpa.repository.JpaRepository;

import java.util.List;

public interface FollowRepository extends JpaRepository<Follow, Follow.FollowId> {

    List<Follow> findByFolloweeId(Long followeeId);

    List<Follow> findByFollowerId(Long followerId);

    long countByFolloweeId(Long followeeId);
}
