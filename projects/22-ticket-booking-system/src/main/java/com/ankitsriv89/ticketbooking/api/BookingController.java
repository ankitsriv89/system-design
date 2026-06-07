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

    @PostMapping
    public ResponseEntity<Booking> checkout(@RequestBody Map<String, String> body) {
        String holdId = body.get("holdId");
        String userId = body.get("userId");
        String amountStr = body.get("amount");
        String idempotencyKey = body.get("idempotencyKey");
        if (holdId == null || userId == null || amountStr == null) {
            return ResponseEntity.badRequest().build();
        }
        Booking booking = bookingService.checkout(holdId, userId, new BigDecimal(amountStr), idempotencyKey);
        return ResponseEntity.status(201).body(booking);
    }

    @GetMapping("/{id}")
    public Booking getBooking(@PathVariable String id) {
        return bookingService.getBooking(id);
    }

    @GetMapping
    public List<Booking> listByUser(@RequestParam String userId) {
        return bookingService.listByUser(userId);
    }
}
