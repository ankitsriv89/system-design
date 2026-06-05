package com.ankitsriv89.proximityservice.repository;

import com.ankitsriv89.proximityservice.domain.VisibilitySetting;
import org.springframework.data.jpa.repository.JpaRepository;

public interface VisibilityRepository extends JpaRepository<VisibilitySetting, String> {
}
