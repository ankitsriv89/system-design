package com.ankitsriv89.newsfeed.service;

import com.ankitsriv89.newsfeed.domain.Follow;
import com.ankitsriv89.newsfeed.repository.FollowRepository;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.Instant;
import java.util.List;

@Service
public class FollowService {

    private final FollowRepository follows;

    public FollowService(FollowRepository follows) {
        this.follows = follows;
    }

    @Transactional
    public void follow(String followerId, String followeeId) {
        if (followerId.equals(followeeId)) {
            throw new IllegalArgumentException("cannot follow yourself");
        }
        // Idempotent: a duplicate follow is a no-op rather than an error.
        if (follows.existsByFollowerIdAndFolloweeId(followerId, followeeId)) {
            return;
        }
        follows.save(new Follow(followerId, followeeId, Instant.now()));
    }

    public List<String> followeesOf(String userId) {
        return follows.findByFollowerId(userId).stream()
                .map(Follow::getFolloweeId)
                .toList();
    }

    public List<String> followersOf(String userId) {
        return follows.findByFolloweeId(userId).stream()
                .map(Follow::getFollowerId)
                .toList();
    }

    public long followerCount(String userId) {
        return follows.countByFolloweeId(userId);
    }
}
