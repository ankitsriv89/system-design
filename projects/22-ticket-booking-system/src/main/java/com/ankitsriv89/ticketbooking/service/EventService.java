package com.ankitsriv89.ticketbooking.service;

import com.ankitsriv89.ticketbooking.domain.Event;
import com.ankitsriv89.ticketbooking.domain.Seat;
import com.ankitsriv89.ticketbooking.repository.EventRepository;
import com.ankitsriv89.ticketbooking.repository.SeatRepository;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.ArrayList;
import java.util.List;
import java.util.NoSuchElementException;

@Service
public class EventService {

    private final EventRepository eventRepo;
    private final SeatRepository seatRepo;

    public EventService(EventRepository eventRepo, SeatRepository seatRepo) {
        this.eventRepo = eventRepo;
        this.seatRepo = seatRepo;
    }

    public List<Event> listEvents() {
        return eventRepo.findAll();
    }

    public Event getEvent(String id) {
        return eventRepo.findById(id)
            .orElseThrow(() -> new NoSuchElementException("Event not found: " + id));
    }

    @Transactional
    public Event createEvent(Event event) {
        Event saved = eventRepo.save(event);
        List<Seat> seats = new ArrayList<>();
        String[] sections = {"A", "B", "C"};
        int seatsPerSection = event.getTotalSeats() / sections.length;
        for (String section : sections) {
            for (int i = 1; i <= seatsPerSection; i++) {
                Seat s = new Seat();
                s.setEventId(saved.getId());
                s.setSection(section);
                s.setRowLabel(section);
                s.setSeatNumber(String.valueOf(i));
                seats.add(s);
            }
        }
        seatRepo.saveAll(seats);
        return saved;
    }
}
