package com.ankitsriv89.ticketbooking.repository;

import com.ankitsriv89.ticketbooking.domain.Hold;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

import java.time.Instant;
import java.util.List;
import java.util.Optional;

public interface HoldRepository extends JpaRepository<Hold, String> {

    Optional<Hold> findBySeatIdAndStatus(String seatId, Hold.Status status);

    @Query("SELECT h FROM Hold h WHERE h.status = 'ACTIVE' AND h.expiresAt < :now")
    List<Hold> findExpiredHolds(@Param("now") Instant now);

    List<Hold> findByUserIdAndEventIdAndStatus(String userId, String eventId, Hold.Status status);

    // Ownership-scoped lookup — collapses 404 vs 403 into a single response.
    Optional<Hold> findByIdAndUserId(String id, String userId);
}
