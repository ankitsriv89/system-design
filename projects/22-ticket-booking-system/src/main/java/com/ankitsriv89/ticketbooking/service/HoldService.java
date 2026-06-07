package com.ankitsriv89.ticketbooking.service;

import com.ankitsriv89.ticketbooking.domain.Hold;
import com.ankitsriv89.ticketbooking.domain.Seat;
import com.ankitsriv89.ticketbooking.repository.HoldRepository;
import com.ankitsriv89.ticketbooking.repository.SeatRepository;
import com.ankitsriv89.ticketbooking.store.HoldStore;
import com.ankitsriv89.ticketbooking.store.SeatMapCache;
import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.MeterRegistry;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.Duration;
import java.time.Instant;
import java.util.List;
import java.util.NoSuchElementException;

@Service
public class HoldService {

    private static final Logger log = LoggerFactory.getLogger(HoldService.class);

    private final SeatRepository seatRepo;
    private final HoldRepository holdRepo;
    private final HoldStore holdStore;
    private final SeatMapCache seatMapCache;
    private final KafkaTemplate<String, Object> kafka;
    private final Counter holdsCreated;
    private final Counter holdsExpired;

    @Value("${ticket-booking.hold-ttl-seconds:300}")
    private int holdTtlSeconds;

    @Value("${ticket-booking.kafka.topic.holds:ticket.holds}")
    private String holdsTopic;

    public HoldService(SeatRepository seatRepo, HoldRepository holdRepo,
                       HoldStore holdStore, SeatMapCache seatMapCache,
                       KafkaTemplate<String, Object> kafka, MeterRegistry registry) {
        this.seatRepo = seatRepo;
        this.holdRepo = holdRepo;
        this.holdStore = holdStore;
        this.seatMapCache = seatMapCache;
        this.kafka = kafka;
        this.holdsCreated = Counter.builder("ticket_booking_holds_created_total").register(registry);
        this.holdsExpired = Counter.builder("ticket_booking_holds_expired_total").register(registry);
    }

    @Transactional
    public Hold createHold(String seatId, String userId) {
        Seat seat = seatRepo.findByIdAndStatus(seatId, Seat.Status.AVAILABLE)
            .orElseThrow(() -> new IllegalStateException("Seat not available: " + seatId));

        // Double-check Redis to catch races not yet reflected in DB
        if (holdStore.isSeatHeld(seatId)) {
            throw new IllegalStateException("Seat already held: " + seatId);
        }

        seat.setStatus(Seat.Status.HELD);
        seatRepo.save(seat);

        Hold hold = new Hold();
        hold.setSeatId(seatId);
        hold.setEventId(seat.getEventId());
        hold.setUserId(userId);
        hold.setExpiresAt(Instant.now().plusSeconds(holdTtlSeconds));
        Hold saved = holdRepo.save(hold);

        holdStore.registerHold(saved.getId(), seatId, Duration.ofSeconds(holdTtlSeconds));
        seatMapCache.evict(seat.getEventId());

        kafka.send(holdsTopic, saved.getEventId(), new HoldEvent("hold.created", saved.getId(), seatId, userId));
        holdsCreated.increment();
        log.info("Hold created hold_id={} seat_id={} user_id={}", saved.getId(), seatId, userId);
        return saved;
    }

    @Scheduled(fixedDelayString = "${ticket-booking.expiry-check-ms:10000}")
    @Transactional
    public void expireHolds() {
        List<Hold> expired = holdRepo.findExpiredHolds(Instant.now());
        for (Hold hold : expired) {
            hold.setStatus(Hold.Status.EXPIRED);
            holdRepo.save(hold);

            seatRepo.findByIdAndStatus(hold.getSeatId(), Seat.Status.HELD).ifPresent(seat -> {
                seat.setStatus(Seat.Status.AVAILABLE);
                seatRepo.save(seat);
                seatMapCache.evict(seat.getEventId());
            });
            holdStore.releaseHold(hold.getId(), hold.getSeatId());
            kafka.send(holdsTopic, hold.getEventId(), new HoldEvent("hold.expired", hold.getId(), hold.getSeatId(), hold.getUserId()));
            holdsExpired.increment();
            log.info("Hold expired hold_id={}", hold.getId());
        }
    }

    public Hold getHold(String holdId) {
        return holdRepo.findById(holdId)
            .orElseThrow(() -> new NoSuchElementException("Hold not found: " + holdId));
    }

    // Ownership-scoped: collapses 404 vs 403 so callers can't probe for other users' holds.
    public Hold getHoldForCaller(String holdId, String callerId) {
        return holdRepo.findByIdAndUserId(holdId, callerId)
            .orElseThrow(() -> new NoSuchElementException("Hold not found: " + holdId));
    }

    public record HoldEvent(String type, String holdId, String seatId, String userId) {}
}
