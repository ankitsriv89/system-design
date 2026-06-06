package com.ankitsriv89.instagram.repository;

import com.ankitsriv89.instagram.domain.Media;
import org.springframework.data.jpa.repository.JpaRepository;

public interface MediaRepository extends JpaRepository<Media, Long> {
}
