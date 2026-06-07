package com.ankitsriv89.ticketbooking.api;

import com.ankitsriv89.ticketbooking.domain.Booking;
import com.ankitsriv89.ticketbooking.service.BookingService;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.math.BigDecimal;
import java.util.List;
import java.util.Map;

@RestController
@RequestMapping("/v1/bookings")
public class BookingController {

    private final BookingService bookingService;

    public BookingController(BookingService bookingService) {
        this.bookingService = bookingService;
    }

    // userId is injected by the upstream auth gateway via X-User-Id header,
    // never trusted from the request body.
    @PostMapping
    public ResponseEntity<Booking> checkout(
            @RequestHeader("X-User-Id") String callerId,
            @RequestBody Map<String, String> body) {
        String holdId = body.get("holdId");
        String amountStr = body.get("amount");
        String idempotencyKey = body.get("idempotencyKey");
        if (holdId == null || amountStr == null) {
            return ResponseEntity.badRequest().build();
        }
        Booking booking = bookingService.checkout(holdId, callerId, new BigDecimal(amountStr), idempotencyKey);
        return ResponseEntity.status(201).body(booking);
    }

    // Ownership-scoped: returns 404 for both missing and unauthorised ids — no existence oracle.
    @GetMapping("/{id}")
    public Booking getBooking(
            @RequestHeader("X-User-Id") String callerId,
            @PathVariable String id) {
        return bookingService.getBookingForCaller(id, callerId);
    }

    // Returns only the caller's own bookings — userId param is ignored.
    @GetMapping
    public List<Booking> listMyBookings(@RequestHeader("X-User-Id") String callerId) {
        return bookingService.listByUser(callerId);
    }
}
