package com.ankitsriv89.proximityservice.repository;

import com.ankitsriv89.proximityservice.domain.Location;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Modifying;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

import java.util.Optional;

public interface LocationRepository extends JpaRepository<Location, Long> {
    Optional<Location> findByUserId(String userId);

    @Modifying
    @Query("DELETE FROM Location l WHERE l.userId = :userId")
    void deleteByUserId(@Param("userId") String userId);
}
