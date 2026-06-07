package com.ankitsriv89.ticketbooking.repository;

import com.ankitsriv89.ticketbooking.domain.Booking;
import org.springframework.data.jpa.repository.JpaRepository;

import java.util.List;
import java.util.Optional;

public interface BookingRepository extends JpaRepository<Booking, String> {

    Optional<Booking> findByIdempotencyKey(String idempotencyKey);

    List<Booking> findByUserId(String userId);

    Optional<Booking> findByHoldId(String holdId);
}
