package com.ankitsriv89.proximityservice.store;

import lombok.RequiredArgsConstructor;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.geo.Circle;
import org.springframework.data.geo.Distance;
import org.springframework.data.geo.GeoResults;
import org.springframework.data.geo.Metrics;
import org.springframework.data.geo.Point;
import org.springframework.data.redis.connection.RedisGeoCommands;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Component;

import java.util.List;

@Component
@RequiredArgsConstructor
public class GeoStore {

    private final StringRedisTemplate redis;

    @Value("${proximity.geo.redis-key:geo:locations}")
    private String geoKey;

    public void upsert(String userId, double lat, double lon) {
        redis.opsForGeo().add(geoKey, new Point(lon, lat), userId);
    }

    public void remove(String userId) {
        redis.opsForGeo().remove(geoKey, userId);
    }

    /** Returns member names within radiusMeters of (lat, lon). */
    public List<String> nearby(double lat, double lon, double radiusMeters) {
        Distance radius = new Distance(radiusMeters / 1000.0, Metrics.KILOMETERS);
        Circle circle = new Circle(new Point(lon, lat), radius);

        RedisGeoCommands.GeoRadiusCommandArgs args = RedisGeoCommands.GeoRadiusCommandArgs
                .newGeoRadiusArgs()
                .includeDistance()
                .sortAscending()
                .limit(200);

        GeoResults<RedisGeoCommands.GeoLocation<String>> results =
                redis.opsForGeo().radius(geoKey, circle, args);

        if (results == null) return List.of();
        return results.getContent().stream()
                .map(r -> r.getContent().getName())
                .toList();
    }
}
