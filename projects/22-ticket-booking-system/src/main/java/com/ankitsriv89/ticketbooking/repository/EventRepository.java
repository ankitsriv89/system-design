package com.ankitsriv89.ticketbooking.repository;

import com.ankitsriv89.ticketbooking.domain.Event;
import org.springframework.data.jpa.repository.JpaRepository;

public interface EventRepository extends JpaRepository<Event, String> {
}
