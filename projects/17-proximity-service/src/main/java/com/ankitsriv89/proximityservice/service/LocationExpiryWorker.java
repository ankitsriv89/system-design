package com.ankitsriv89.proximityservice.service;

import com.ankitsriv89.proximityservice.repository.LocationRepository;
import com.ankitsriv89.proximityservice.store.GeoStore;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;
import org.springframework.transaction.annotation.Transactional;

import java.time.Instant;
import java.util.Map;

/**
 * Periodically removes stale locations (no update within TTL window).
 * Publishes location.expired events for each evicted entry.
 */
@Component
@RequiredArgsConstructor
@Slf4j
public class LocationExpiryWorker {

    private final LocationRepository locationRepo;
    private final GeoStore geoStore;
    private final KafkaTemplate<String, Object> kafka;

    @Value("${proximity.location.ttl-seconds:300}")
    private long ttlSeconds;

    @Value("${proximity.kafka.topics.location-expired:location.expired}")
    private String locationExpiredTopic;

    @Scheduled(fixedDelayString = "${proximity.location.ttl-seconds:300}000")
    @Transactional
    public void evictStale() {
        Instant cutoff = Instant.now().minusSeconds(ttlSeconds);
        var stale = locationRepo.findAll().stream()
                .filter(loc -> loc.getUpdatedAt().isBefore(cutoff))
                .toList();

        stale.forEach(loc -> {
            locationRepo.delete(loc);
            geoStore.remove(loc.getUserId());
            kafka.send(locationExpiredTopic, loc.getUserId(),
                    Map.of("userId", loc.getUserId(), "reason", "ttl_expired"));
            log.info("Evicted stale location userId={}", loc.getUserId());
        });

        if (!stale.isEmpty()) {
            log.info("Eviction run complete count={}", stale.size());
        }
    }
}
