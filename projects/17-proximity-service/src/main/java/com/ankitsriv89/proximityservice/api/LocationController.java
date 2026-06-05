package com.ankitsriv89.proximityservice.api;

import com.ankitsriv89.proximityservice.domain.Location;
import com.ankitsriv89.proximityservice.domain.VisibilityMode;
import com.ankitsriv89.proximityservice.domain.VisibilitySetting;
import com.ankitsriv89.proximityservice.service.LocationService;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.slf4j.MDC;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.List;
import java.util.Map;
import java.util.UUID;

@RestController
@RequestMapping("/v1")
@RequiredArgsConstructor
@Slf4j
public class LocationController {

    private final LocationService locationService;

    @PostMapping("/locations")
    public ResponseEntity<Location> updateLocation(@RequestBody UpdateLocationRequest req) {
        MDC.put("requestId", UUID.randomUUID().toString());
        Location loc = locationService.updateLocation(req.userId(), req.lat(), req.lng());
        return ResponseEntity.ok(loc);
    }

    @GetMapping("/nearby")
    public ResponseEntity<Map<String, Object>> nearby(
            @RequestParam String userId,
            @RequestParam double lat,
            @RequestParam double lng,
            @RequestParam(defaultValue = "1000") double radiusMeters) {
        MDC.put("requestId", UUID.randomUUID().toString());
        List<String> results = locationService.findNearby(userId, lat, lng, radiusMeters);
        return ResponseEntity.ok(Map.of("nearby", results, "count", results.size()));
    }

    @DeleteMapping("/locations/{userId}")
    public ResponseEntity<Void> deleteLocation(@PathVariable String userId) {
        MDC.put("requestId", UUID.randomUUID().toString());
        locationService.deleteLocation(userId);
        return ResponseEntity.noContent().build();
    }

    @PutMapping("/visibility/{userId}")
    public ResponseEntity<VisibilitySetting> updateVisibility(
            @PathVariable String userId,
            @RequestBody Map<String, String> body) {
        VisibilityMode mode = VisibilityMode.valueOf(body.getOrDefault("mode", "FRIENDS").toUpperCase());
        return ResponseEntity.ok(locationService.updateVisibility(userId, mode));
    }

    @GetMapping("/locations/{userId}")
    public ResponseEntity<Location> getLocation(@PathVariable String userId) {
        return locationService.getLocation(userId)
                .map(ResponseEntity::ok)
                .orElse(ResponseEntity.notFound().build());
    }

    record UpdateLocationRequest(String userId, double lat, double lng) {}
}
