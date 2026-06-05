package com.ankitsriv89.proximityservice.service;

import com.ankitsriv89.proximityservice.domain.GeohashUtil;
import com.ankitsriv89.proximityservice.domain.Location;
import com.ankitsriv89.proximityservice.domain.VisibilityMode;
import com.ankitsriv89.proximityservice.domain.VisibilitySetting;
import com.ankitsriv89.proximityservice.repository.LocationRepository;
import com.ankitsriv89.proximityservice.repository.VisibilityRepository;
import com.ankitsriv89.proximityservice.store.GeoStore;
import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.core.instrument.Timer;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;
import java.util.Map;
import java.util.Optional;

@Service
@Slf4j
public class LocationService {

    private final LocationRepository locationRepo;
    private final VisibilityRepository visibilityRepo;
    private final GeoStore geoStore;
    private final KafkaTemplate<String, Object> kafka;
    private final Counter locationUpdates;
    private final Counter nearbyQueries;
    private final Timer nearbyLatency;

    @Value("${proximity.geo.default-radius-m:1000}")
    private double defaultRadiusMeters;

    @Value("${proximity.kafka.topics.location-updated:location.updated}")
    private String locationUpdatedTopic;

    @Value("${proximity.kafka.topics.nearby-queried:nearby.queried}")
    private String nearbyQueriedTopic;

    @Value("${proximity.kafka.topics.location-expired:location.expired}")
    private String locationExpiredTopic;

    public LocationService(LocationRepository locationRepo, VisibilityRepository visibilityRepo,
                           GeoStore geoStore, KafkaTemplate<String, Object> kafka,
                           MeterRegistry registry) {
        this.locationRepo = locationRepo;
        this.visibilityRepo = visibilityRepo;
        this.geoStore = geoStore;
        this.kafka = kafka;
        this.locationUpdates = Counter.builder("proximity_location_updates_total")
                .description("Total location updates").register(registry);
        this.nearbyQueries = Counter.builder("proximity_nearby_queries_total")
                .description("Total nearby queries").register(registry);
        this.nearbyLatency = Timer.builder("proximity_nearby_query_latency_seconds")
                .description("Nearby query latency").register(registry);
    }

    @Transactional
    public Location updateLocation(String userId, double lat, double lng) {
        String geohash = GeohashUtil.encode(lat, lng);

        Location loc = locationRepo.findByUserId(userId)
                .orElse(new Location(userId, lat, lng, geohash));
        loc.setLat(lat);
        loc.setLng(lng);
        loc.setGeohash(geohash);
        loc.setUpdatedAt(java.time.Instant.now());

        locationRepo.save(loc);
        geoStore.upsert(userId, lat, lng);

        locationUpdates.increment();
        kafka.send(locationUpdatedTopic, userId, Map.of(
                "userId", userId, "lat", lat, "lng", lng, "geohash", geohash
        ));
        log.debug("Location updated userId={} geohash={}", userId, geohash);
        return loc;
    }

    public List<String> findNearby(String requesterId, double lat, double lng, double radiusMeters) {
        return nearbyLatency.record(() -> {
            List<String> candidates = geoStore.nearby(lat, lng, radiusMeters);

            List<String> visible = candidates.stream()
                    .filter(uid -> !uid.equals(requesterId))
                    .filter(uid -> isVisible(uid))
                    .toList();

            nearbyQueries.increment();
            kafka.send(nearbyQueriedTopic, requesterId, Map.of(
                    "requesterId", requesterId, "lat", lat, "lng", lng,
                    "radiusMeters", radiusMeters, "resultCount", visible.size()
            ));
            return visible;
        });
    }

    @Transactional
    public void deleteLocation(String userId) {
        locationRepo.deleteByUserId(userId);
        geoStore.remove(userId);
        kafka.send(locationExpiredTopic, userId, Map.of("userId", userId));
        log.info("Location deleted userId={}", userId);
    }

    @Transactional
    public VisibilitySetting updateVisibility(String userId, VisibilityMode mode) {
        VisibilitySetting vs = visibilityRepo.findById(userId)
                .orElse(new VisibilitySetting(userId, mode));
        vs.setMode(mode);
        vs.setUpdatedAt(java.time.Instant.now());
        return visibilityRepo.save(vs);
    }

    public Optional<Location> getLocation(String userId) {
        return locationRepo.findByUserId(userId);
    }

    private boolean isVisible(String userId) {
        return visibilityRepo.findById(userId)
                .map(vs -> vs.getMode() != VisibilityMode.HIDDEN)
                .orElse(true);
    }
}
