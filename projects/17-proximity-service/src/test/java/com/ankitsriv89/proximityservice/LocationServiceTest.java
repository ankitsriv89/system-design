package com.ankitsriv89.proximityservice;

import com.ankitsriv89.proximityservice.domain.Location;
import com.ankitsriv89.proximityservice.domain.VisibilityMode;
import com.ankitsriv89.proximityservice.repository.LocationRepository;
import com.ankitsriv89.proximityservice.repository.VisibilityRepository;
import com.ankitsriv89.proximityservice.service.LocationService;
import com.ankitsriv89.proximityservice.store.GeoStore;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.kafka.core.KafkaTemplate;

import java.util.List;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

class LocationServiceTest {

    private LocationRepository locationRepo;
    private VisibilityRepository visibilityRepo;
    private GeoStore geoStore;
    private KafkaTemplate<String, Object> kafka;
    private LocationService service;

    @BeforeEach
    void setUp() {
        locationRepo = mock(LocationRepository.class);
        visibilityRepo = mock(VisibilityRepository.class);
        geoStore = mock(GeoStore.class);
        kafka = mock(KafkaTemplate.class);
        service = new LocationService(locationRepo, visibilityRepo, geoStore, kafka,
                new SimpleMeterRegistry());
    }

    @Test
    void updateLocationPersistsAndPublishes() {
        when(locationRepo.findByUserId("u1")).thenReturn(Optional.empty());
        when(locationRepo.save(any())).thenAnswer(inv -> inv.getArgument(0));

        Location loc = service.updateLocation("u1", 37.7749, -122.4194);

        assertEquals("u1", loc.getUserId());
        assertNotNull(loc.getGeohash());
        verify(geoStore).upsert("u1", 37.7749, -122.4194);
        verify(kafka).send(anyString(), eq("u1"), any());
    }

    @Test
    void findNearbyFiltersHiddenUsers() {
        when(geoStore.nearby(anyDouble(), anyDouble(), anyDouble()))
                .thenReturn(List.of("u2", "u3"));
        when(visibilityRepo.findById("u2")).thenReturn(Optional.empty());
        when(visibilityRepo.findById("u3")).thenReturn(
                Optional.of(new com.ankitsriv89.proximityservice.domain.VisibilitySetting("u3", VisibilityMode.HIDDEN)));

        List<String> results = service.findNearby("u1", 37.7749, -122.4194, 1000);

        assertEquals(List.of("u2"), results);
    }
}
