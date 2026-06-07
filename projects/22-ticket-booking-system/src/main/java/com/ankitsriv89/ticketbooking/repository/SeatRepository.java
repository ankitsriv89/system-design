package com.ankitsriv89.ticketbooking.repository;

import com.ankitsriv89.ticketbooking.domain.Seat;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

import java.util.List;
import java.util.Optional;

public interface SeatRepository extends JpaRepository<Seat, String> {

    List<Seat> findByEventId(String eventId);

    List<Seat> findByEventIdAndStatus(String eventId, Seat.Status status);

    @Query("SELECT s FROM Seat s WHERE s.id = :id AND s.status = :status")
    Optional<Seat> findByIdAndStatus(@Param("id") String id, @Param("status") Seat.Status status);

    long countByEventIdAndStatus(String eventId, Seat.Status status);
}
