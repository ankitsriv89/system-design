package com.ankitsriv89.twitter.service;

import com.ankitsriv89.twitter.domain.Tweet;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import java.time.Duration;
import java.time.Instant;

/**
 * Computes a timeline ranking score for a tweet. The MVP uses a transparent
 * time-decay model so the home timeline is "ranked, not purely
 * reverse-chronological" while staying explainable for the tutorial.
 *
 * <p>score = 0.5 ^ (ageHours / halfLifeHours): a brand-new tweet scores ~1.0;
 * after one half-life it scores exactly 0.5, after two 0.25, and so on. This
 * single score is stored as the ZSET score in Redis so the timeline stays in
 * ranked order without re-sorting on read. A production ranker would blend in
 * engagement signals (likes, retweets, author affinity); this is where those
 * terms would be added.
 */
@Service
public class RankingService {

    private final double halfLifeHours;

    public RankingService(@Value("${twitter.ranking.half-life-hours}") double halfLifeHours) {
        this.halfLifeHours = halfLifeHours;
    }

    public double score(Tweet tweet, Instant now) {
        return score(tweet.getCreatedAt(), now);
    }

    public double score(Instant createdAt, Instant now) {
        double ageHours = Math.max(0, Duration.between(createdAt, now).toMillis() / 3_600_000.0);
        return Math.exp(-Math.log(2) * ageHours / halfLifeHours);
    }
}
