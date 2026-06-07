package com.ankitsriv89.ticketbooking.service;

import com.ankitsriv89.ticketbooking.domain.Seat;
import com.ankitsriv89.ticketbooking.repository.SeatRepository;
import com.ankitsriv89.ticketbooking.store.SeatMapCache;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Service;

import java.util.List;
import java.util.Map;

@Service
public class SeatService {

    private final SeatRepository seatRepo;
    private final SeatMapCache cache;
    private final ObjectMapper mapper;

    public SeatService(SeatRepository seatRepo, SeatMapCache cache, ObjectMapper mapper) {
        this.seatRepo = seatRepo;
        this.cache = cache;
        this.mapper = mapper;
    }

    public List<Seat> getSeats(String eventId) {
        String cached = cache.get(eventId);
        if (cached != null) {
            try {
                return mapper.readValue(cached, new TypeReference<>() {});
            } catch (Exception ignored) {}
        }
        List<Seat> seats = seatRepo.findByEventId(eventId);
        try {
            cache.put(eventId, mapper.writeValueAsString(seats));
        } catch (Exception ignored) {}
        return seats;
    }

    public Map<String, Long> getSeatStats(String eventId) {
        return Map.of(
            "available", seatRepo.countByEventIdAndStatus(eventId, Seat.Status.AVAILABLE),
            "held", seatRepo.countByEventIdAndStatus(eventId, Seat.Status.HELD),
            "booked", seatRepo.countByEventIdAndStatus(eventId, Seat.Status.BOOKED)
        );
    }
}
