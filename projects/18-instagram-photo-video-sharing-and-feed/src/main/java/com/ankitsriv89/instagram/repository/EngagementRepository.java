package com.ankitsriv89.instagram.repository;

import com.ankitsriv89.instagram.domain.Engagement;
import org.springframework.data.jpa.repository.JpaRepository;

public interface EngagementRepository extends JpaRepository<Engagement, Engagement.EngagementId> {

    long countByPostIdAndType(Long postId, String type);

    boolean existsByPostIdAndUserIdAndType(Long postId, Long userId, String type);
}
