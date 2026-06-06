package com.ankitsriv89.instagram.service;

import com.ankitsriv89.instagram.domain.Engagement;
import com.ankitsriv89.instagram.dto.PostLikedEvent;
import com.ankitsriv89.instagram.repository.EngagementRepository;
import com.ankitsriv89.instagram.store.CounterStore;
import com.ankitsriv89.instagram.store.EventPublisher;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/**
 * Likes/engagement. Postgres holds the durable engagement rows (source of
 * truth); Redis holds the hot counter. Operations are idempotent: liking twice
 * is a no-op, so the counter never drifts.
 */
@Service
public class EngagementService {

    private static final Logger log = LoggerFactory.getLogger(EngagementService.class);
    private static final String LIKE = "LIKE";

    private final EngagementRepository engagementRepository;
    private final CounterStore counterStore;
    private final EventPublisher eventPublisher;

    public EngagementService(EngagementRepository engagementRepository,
                             CounterStore counterStore,
                             EventPublisher eventPublisher) {
        this.engagementRepository = engagementRepository;
        this.counterStore = counterStore;
        this.eventPublisher = eventPublisher;
    }

    @Transactional
    public long like(Long postId, Long userId) {
        if (engagementRepository.existsByPostIdAndUserIdAndType(postId, userId, LIKE)) {
            return counterStore.likeCount(postId); // idempotent
        }
        engagementRepository.save(new Engagement(postId, userId, LIKE));
        long count = counterStore.incrementLikes(postId);
        eventPublisher.publishPostLiked(new PostLikedEvent(postId, userId, true));
        log.debug("like post={} user={} count={}", postId, userId, count);
        return count;
    }

    @Transactional
    public long unlike(Long postId, Long userId) {
        if (!engagementRepository.existsByPostIdAndUserIdAndType(postId, userId, LIKE)) {
            return counterStore.likeCount(postId); // idempotent
        }
        engagementRepository.deleteById(new Engagement.EngagementId(postId, userId, LIKE));
        long count = counterStore.decrementLikes(postId);
        eventPublisher.publishPostLiked(new PostLikedEvent(postId, userId, false));
        log.debug("unlike post={} user={} count={}", postId, userId, count);
        return count;
    }

    @Transactional(readOnly = true)
    public long likeCount(Long postId) {
        return counterStore.likeCount(postId);
    }
}
