package com.ankitsriv89.newsfeed;

import com.ankitsriv89.newsfeed.service.RankingService;
import org.junit.jupiter.api.Test;

import java.time.Duration;
import java.time.Instant;

import static org.junit.jupiter.api.Assertions.*;

class RankingServiceTest {

    private final RankingService ranking = new RankingService(12.0); // 12h half-life

    @Test
    void freshPostScoresNearOne() {
        Instant now = Instant.now();
        assertEquals(1.0, ranking.score(now, now), 1e-9);
    }

    @Test
    void scoreHalvesAfterOneHalfLife() {
        Instant now = Instant.now();
        Instant twelveHoursAgo = now.minus(Duration.ofHours(12));
        assertEquals(0.5, ranking.score(twelveHoursAgo, now), 1e-3);
    }

    @Test
    void olderPostScoresLowerThanNewer() {
        Instant now = Instant.now();
        double newer = ranking.score(now.minus(Duration.ofHours(1)), now);
        double older = ranking.score(now.minus(Duration.ofHours(10)), now);
        assertTrue(newer > older, "more recent post must rank higher");
    }

    @Test
    void futureClockSkewDoesNotExceedOne() {
        Instant now = Instant.now();
        Instant future = now.plus(Duration.ofHours(5));
        assertTrue(ranking.score(future, now) <= 1.0);
    }
}
