package com.ankitsriv89.twitter.service;

import com.ankitsriv89.twitter.domain.Follow;
import com.ankitsriv89.twitter.repository.FollowRepository;
import org.springframework.stereotype.Service;

import java.time.Instant;
import java.util.List;

/**
 * The social graph: who follows whom. Backed by PostgreSQL. The follower set of
 * an author is the fanout-on-write target list; the followee set of a user is
 * the read-path pull + backfill source.
 */
@Service
public class FollowService {

    private final FollowRepository follows;

    public FollowService(FollowRepository follows) {
        this.follows = follows;
    }

    public void follow(String followerId, String followeeId) {
        if (followerId.equals(followeeId)) {
            throw new IllegalArgumentException("cannot follow yourself");
        }
        if (follows.existsByFollowerIdAndFolloweeId(followerId, followeeId)) {
            return;     // idempotent: following twice is a no-op
        }
        follows.save(new Follow(followerId, followeeId, Instant.now()));
    }

    /** Followers of an author — the fanout-on-write targets. */
    public List<String> followersOf(String authorId) {
        return follows.findByFolloweeId(authorId).stream()
                .map(Follow::getFollowerId)
                .toList();
    }

    /** Who a user follows — the read-path pull + backfill source. */
    public List<String> followeesOf(String userId) {
        return follows.findByFollowerId(userId).stream()
                .map(Follow::getFolloweeId)
                .toList();
    }

    public long followerCount(String authorId) {
        return follows.countByFolloweeId(authorId);
    }
}
