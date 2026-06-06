package com.ankitsriv89.instagram.service;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import java.time.Duration;
import java.time.Instant;

/**
 * Feed ranking score. Transparent time-decay model: a brand-new post scores
 * ~1.0; after one half-life it scores 0.5, after two 0.25, etc. The score is
 * stored as the Redis ZSET score so timelines stay ranked without re-sorting on
 * read. Engagement signals (likes, affinity) would be blended in here in a real
 * system.
 */
@Service
public class RankingService {

    private final double halfLifeHours;

    public RankingService(@Value("${instagram.feed.half-life-hours}") double halfLifeHours) {
        this.halfLifeHours = halfLifeHours;
    }

    public double score(Instant createdAt, Instant now) {
        double ageHours = Math.max(0, Duration.between(createdAt, now).toMillis() / 3_600_000.0);
        return Math.exp(-Math.log(2) * ageHours / halfLifeHours);
    }
}
