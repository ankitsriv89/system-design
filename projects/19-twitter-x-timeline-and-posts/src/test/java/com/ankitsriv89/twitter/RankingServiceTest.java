package com.ankitsriv89.twitter;

import com.ankitsriv89.twitter.service.RankingService;
import com.ankitsriv89.twitter.store.SearchStore;
import org.junit.jupiter.api.Test;

import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Pure-logic unit tests — no Spring context, so they run fast and need no
 * Postgres/Redis/Kafka/OpenSearch. Cover the two pieces of non-trivial logic:
 * the time-decay ranking score and hashtag extraction for trends.
 */
class RankingServiceTest {

    private final RankingService ranking = new RankingService(12.0);   // 12h half-life

    @Test
    void freshTweetScoresNearOne() {
        Instant now = Instant.now();
        double score = ranking.score(now, now);
        assertEquals(1.0, score, 1e-6);
    }

    @Test
    void scoreHalvesEveryHalfLife() {
        Instant now = Instant.now();
        Instant twelveHoursAgo = now.minus(12, ChronoUnit.HOURS);
        Instant twentyFourHoursAgo = now.minus(24, ChronoUnit.HOURS);

        assertEquals(0.5, ranking.score(twelveHoursAgo, now), 1e-3);
        assertEquals(0.25, ranking.score(twentyFourHoursAgo, now), 1e-3);
    }

    @Test
    void olderTweetsScoreLower() {
        Instant now = Instant.now();
        double fresh = ranking.score(now.minus(1, ChronoUnit.HOURS), now);
        double stale = ranking.score(now.minus(48, ChronoUnit.HOURS), now);
        assertTrue(fresh > stale);
    }

    @Test
    void extractsLowercasedHashtags() {
        List<String> tags = SearchStore.extractHashtags("Loving #SystemDesign and #Kafka #kafka today!");
        assertTrue(tags.contains("systemdesign"));
        // #Kafka and #kafka both normalize to "kafka" — appears twice, counted by aggregation.
        assertEquals(2, tags.stream().filter("kafka"::equals).count());
    }

    @Test
    void noHashtagsYieldsEmptyList() {
        assertTrue(SearchStore.extractHashtags("plain tweet with no tags").isEmpty());
    }
}
