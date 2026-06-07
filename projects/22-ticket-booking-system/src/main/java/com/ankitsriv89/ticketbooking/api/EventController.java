package com.ankitsriv89.ticketbooking.api;

import com.ankitsriv89.ticketbooking.domain.Event;
import com.ankitsriv89.ticketbooking.service.EventService;
import com.ankitsriv89.ticketbooking.service.SeatService;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.List;
import java.util.Map;

@RestController
@RequestMapping("/v1/events")
public class EventController {

    private final EventService eventService;
    private final SeatService seatService;

    public EventController(EventService eventService, SeatService seatService) {
        this.eventService = eventService;
        this.seatService = seatService;
    }

    @GetMapping
    public List<Event> listEvents() {
        return eventService.listEvents();
    }

    @GetMapping("/{id}")
    public Event getEvent(@PathVariable String id) {
        return eventService.getEvent(id);
    }

    @PostMapping
    public ResponseEntity<Event> createEvent(@RequestBody Event event) {
        return ResponseEntity.status(201).body(eventService.createEvent(event));
    }

    @GetMapping("/{id}/seats")
    public ResponseEntity<?> getSeats(@PathVariable String id) {
        return ResponseEntity.ok(seatService.getSeats(id));
    }

    @GetMapping("/{id}/seats/stats")
    public Map<String, Long> getSeatStats(@PathVariable String id) {
        return seatService.getSeatStats(id);
    }
}
