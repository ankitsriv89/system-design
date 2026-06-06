package com.ankitsriv89.instagram.service;

import com.ankitsriv89.instagram.domain.Follow;
import com.ankitsriv89.instagram.repository.FollowRepository;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;

/** Social-graph reads/writes. The graph lives in Postgres (durable, queryable). */
@Service
public class FollowService {

    private final FollowRepository followRepository;

    public FollowService(FollowRepository followRepository) {
        this.followRepository = followRepository;
    }

    @Transactional
    public void follow(Long followerId, Long followeeId) {
        if (followerId.equals(followeeId)) {
            throw new IllegalArgumentException("cannot follow yourself");
        }
        followRepository.save(new Follow(followerId, followeeId));
    }

    @Transactional
    public void unfollow(Long followerId, Long followeeId) {
        followRepository.deleteById(new Follow.FollowId(followerId, followeeId));
    }

    @Transactional(readOnly = true)
    public List<Long> followersOf(Long userId) {
        return followRepository.findByFolloweeId(userId).stream()
                .map(Follow::getFollowerId)
                .toList();
    }

    @Transactional(readOnly = true)
    public List<Long> followingOf(Long userId) {
        return followRepository.findByFollowerId(userId).stream()
                .map(Follow::getFolloweeId)
                .toList();
    }

    @Transactional(readOnly = true)
    public long followerCount(Long userId) {
        return followRepository.countByFolloweeId(userId);
    }
}
