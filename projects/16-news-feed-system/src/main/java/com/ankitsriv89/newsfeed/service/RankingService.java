package com.ankitsriv89.newsfeed.service;

import com.ankitsriv89.newsfeed.domain.Post;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import java.time.Duration;
import java.time.Instant;

/**
 * Computes a feed ranking score for a post. The MVP uses a transparent
 * time-decay model so the feed is "ranked, not purely reverse-chronological"
 * while staying explainable for the tutorial.
 *
 * <p>score = 0.5 ^ (ageHours / halfLifeHours), so a brand-new post scores ~1.0;
 * after one half-life it scores exactly 0.5, after two 0.25, and so on. This
 * single score is stored as the ZSET score in Redis
 * so the timeline is kept in ranked order without re-sorting on read. In a real
 * system you would blend in engagement signals (likes, affinity, dwell); the
 * structure here is where those terms would be added.
 */
@Service
public class RankingService {

    private final double halfLifeHours;

    public RankingService(@Value("${newsfeed.ranking.half-life-hours}") double halfLifeHours) {
        this.halfLifeHours = halfLifeHours;
    }

    public double score(Post post, Instant now) {
        return score(post.getCreatedAt(), now);
    }

    public double score(Instant createdAt, Instant now) {
        double ageHours = Math.max(0, Duration.between(createdAt, now).toMillis() / 3_600_000.0);
        // 0.5^(age/halfLife): a true half-life so the score halves every halfLifeHours.
        return Math.exp(-Math.log(2) * ageHours / halfLifeHours);
    }
}
