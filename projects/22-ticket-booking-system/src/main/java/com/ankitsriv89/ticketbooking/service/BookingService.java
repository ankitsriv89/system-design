package com.ankitsriv89.ticketbooking.service;

import com.ankitsriv89.ticketbooking.domain.Booking;
import com.ankitsriv89.ticketbooking.domain.Hold;
import com.ankitsriv89.ticketbooking.domain.Seat;
import com.ankitsriv89.ticketbooking.repository.BookingRepository;
import com.ankitsriv89.ticketbooking.repository.HoldRepository;
import com.ankitsriv89.ticketbooking.repository.SeatRepository;
import com.ankitsriv89.ticketbooking.store.HoldStore;
import com.ankitsriv89.ticketbooking.store.SeatMapCache;
import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.core.instrument.Timer;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.math.BigDecimal;
import java.time.Instant;
import java.util.List;
import java.util.NoSuchElementException;

@Service
public class BookingService {

    private static final Logger log = LoggerFactory.getLogger(BookingService.class);

    private final BookingRepository bookingRepo;
    private final HoldRepository holdRepo;
    private final SeatRepository seatRepo;
    private final HoldStore holdStore;
    private final SeatMapCache seatMapCache;
    private final KafkaTemplate<String, Object> kafka;
    private final Counter bookingsConfirmed;
    private final Counter paymentsFailed;
    private final Timer checkoutTimer;

    @Value("${ticket-booking.kafka.topic.bookings:ticket.bookings}")
    private String bookingsTopic;

    public BookingService(BookingRepository bookingRepo, HoldRepository holdRepo,
                          SeatRepository seatRepo, HoldStore holdStore,
                          SeatMapCache seatMapCache, KafkaTemplate<String, Object> kafka,
                          MeterRegistry registry) {
        this.bookingRepo = bookingRepo;
        this.holdRepo = holdRepo;
        this.seatRepo = seatRepo;
        this.holdStore = holdStore;
        this.seatMapCache = seatMapCache;
        this.kafka = kafka;
        this.bookingsConfirmed = Counter.builder("ticket_booking_confirmed_total").register(registry);
        this.paymentsFailed = Counter.builder("ticket_booking_payment_failed_total").register(registry);
        this.checkoutTimer = Timer.builder("ticket_booking_checkout_seconds").register(registry);
    }

    @Transactional
    public Booking checkout(String holdId, String userId, BigDecimal amount, String idempotencyKey) {
        // Idempotency guard
        if (idempotencyKey != null) {
            var existing = bookingRepo.findByIdempotencyKey(idempotencyKey);
            if (existing.isPresent()) return existing.get();
        }

        return checkoutTimer.record(() -> {
            Hold hold = holdRepo.findById(holdId)
                .orElseThrow(() -> new NoSuchElementException("Hold not found: " + holdId));

            if (!hold.getUserId().equals(userId)) {
                throw new IllegalStateException("Hold does not belong to user");
            }
            if (hold.getStatus() != Hold.Status.ACTIVE) {
                throw new IllegalStateException("Hold is not active: " + hold.getStatus());
            }
            if (hold.getExpiresAt().isBefore(Instant.now())) {
                throw new IllegalStateException("Hold has expired");
            }

            Booking booking = new Booking();
            booking.setHoldId(holdId);
            booking.setSeatId(hold.getSeatId());
            booking.setEventId(hold.getEventId());
            booking.setUserId(userId);
            booking.setAmount(amount);
            booking.setIdempotencyKey(idempotencyKey);
            booking.setPaymentStatus(Booking.PaymentStatus.PENDING);
            Booking saved = bookingRepo.save(booking);

            // Simulate payment — mock always succeeds unless amount is 0
            boolean paymentOk = amount.compareTo(BigDecimal.ZERO) > 0;

            if (paymentOk) {
                saved.setPaymentStatus(Booking.PaymentStatus.COMPLETED);
                saved.setUpdatedAt(Instant.now());
                bookingRepo.save(saved);

                hold.setStatus(Hold.Status.CONVERTED);
                holdRepo.save(hold);

                Seat seat = seatRepo.findById(hold.getSeatId())
                    .orElseThrow(() -> new IllegalStateException("Seat missing: " + hold.getSeatId()));
                seat.setStatus(Seat.Status.BOOKED);
                seatRepo.save(seat);

                holdStore.releaseHold(holdId, hold.getSeatId());
                seatMapCache.evict(hold.getEventId());

                kafka.send(bookingsTopic, hold.getEventId(),
                    new BookingEvent("booking.confirmed", saved.getId(), hold.getSeatId(), userId));
                bookingsConfirmed.increment();
                log.info("Booking confirmed booking_id={} seat_id={}", saved.getId(), hold.getSeatId());
            } else {
                saved.setPaymentStatus(Booking.PaymentStatus.FAILED);
                saved.setUpdatedAt(Instant.now());
                bookingRepo.save(saved);
                paymentsFailed.increment();
                kafka.send(bookingsTopic, hold.getEventId(),
                    new BookingEvent("payment.failed", saved.getId(), hold.getSeatId(), userId));
                log.warn("Payment failed booking_id={}", saved.getId());
                throw new IllegalStateException("Payment failed");
            }

            return saved;
        });
    }

    public List<Booking> listByUser(String userId) {
        return bookingRepo.findByUserId(userId);
    }

    public Booking getBooking(String id) {
        return bookingRepo.findById(id)
            .orElseThrow(() -> new NoSuchElementException("Booking not found: " + id));
    }

    // Ownership-scoped: always returns 404 whether the id doesn't exist or belongs
    // to a different user — avoids leaking resource existence to unauthorised callers.
    public Booking getBookingForCaller(String id, String callerId) {
        return bookingRepo.findByIdAndUserId(id, callerId)
            .orElseThrow(() -> new NoSuchElementException("Booking not found: " + id));
    }

    public record BookingEvent(String type, String bookingId, String seatId, String userId) {}
}
