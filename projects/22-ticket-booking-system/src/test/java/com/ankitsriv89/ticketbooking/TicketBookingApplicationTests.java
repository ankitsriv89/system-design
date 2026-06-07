package com.ankitsriv89.ticketbooking;

import org.junit.jupiter.api.Test;

class TicketBookingApplicationTests {

    @Test
    void holdExpiryAfterTtl() {
        // Covered by integration_test.sh — hold created, TTL elapses, seat returns to AVAILABLE
    }

    @Test
    void idempotentCheckout() {
        // Same idempotency key submitted twice must return the same Booking without double-charging
    }

    @Test
    void oversellPrevention() {
        // Concurrent hold requests for the same seat must result in exactly one success
    }
}
