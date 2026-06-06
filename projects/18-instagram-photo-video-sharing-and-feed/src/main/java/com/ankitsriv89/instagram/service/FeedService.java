package com.ankitsriv89.instagram.service;

import com.ankitsriv89.instagram.domain.Media;
import com.ankitsriv89.instagram.domain.Post;
import com.ankitsriv89.instagram.dto.FeedItemResponse;
import com.ankitsriv89.instagram.repository.PostRepository;
import com.ankitsriv89.instagram.store.CounterStore;
import com.ankitsriv89.instagram.store.TimelineStore;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.domain.PageRequest;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.Instant;
import java.util.*;

/**
 * Milestone 3 — hybrid feed read.
 *
 * <p>The bulk of a user's feed is the precomputed Redis timeline (push fanout).
 * Celebrity authors (whom the user follows but who were skipped during fanout)
 * are <em>pulled</em> live: their recent posts are fetched from Postgres and
 * merged in. Everything is then ranked by score and paginated.
 */
@Service
public class FeedService {

    private final TimelineStore timelines;
    private final FollowService follows;
    private final PostRepository postRepository;
    private final MediaService mediaService;
    private final CounterStore counters;
    private final RankingService ranking;
    private final MediaUrlService mediaUrls;
    private final long celebrityThreshold;
    private final int pageSize;

    public FeedService(TimelineStore timelines,
                       FollowService follows,
                       PostRepository postRepository,
                       MediaService mediaService,
                       CounterStore counters,
                       RankingService ranking,
                       MediaUrlService mediaUrls,
                       @Value("${instagram.feed.fanout-follower-threshold}") long celebrityThreshold,
                       @Value("${instagram.feed.page-size}") int pageSize) {
        this.timelines = timelines;
        this.follows = follows;
        this.postRepository = postRepository;
        this.mediaService = mediaService;
        this.counters = counters;
        this.ranking = ranking;
        this.mediaUrls = mediaUrls;
        this.celebrityThreshold = celebrityThreshold;
        this.pageSize = pageSize;
    }

    @Transactional(readOnly = true)
    public List<FeedItemResponse> feed(Long userId, int limit) {
        int n = limit > 0 ? limit : pageSize;
        Instant now = Instant.now();

        // 1. Precomputed push timeline: postId -> score.
        Map<Long, Double> scored = new LinkedHashMap<>(timelines.topN(userId, n * 2));

        // 2. Celebrity pull: authors the user follows whose posts were never
        //    fanned out. Fetch their recent posts live and score them now.
        List<Long> celebrities = follows.followingOf(userId).stream()
                .filter(a -> follows.followerCount(a) > celebrityThreshold)
                .toList();
        if (!celebrities.isEmpty()) {
            List<Post> recent = postRepository.findByUserIdInOrderByCreatedAtDesc(
                    celebrities, PageRequest.of(0, n));
            for (Post p : recent) {
                scored.putIfAbsent(p.getId(), ranking.score(p.getCreatedAt(), now));
            }
        }

        // 3. Rank by score desc, take top-n, hydrate.
        return scored.entrySet().stream()
                .sorted(Map.Entry.<Long, Double>comparingByValue().reversed())
                .limit(n)
                .map(e -> hydrate(e.getKey(), e.getValue()))
                .filter(Objects::nonNull)
                .toList();
    }

    private FeedItemResponse hydrate(Long postId, double score) {
        Post post = postRepository.findById(postId).orElse(null);
        if (post == null) {
            return null; // post deleted since it entered the timeline
        }
        Media media = mediaService.get(post.getMediaId());
        return new FeedItemResponse(
                post.getId(), post.getUserId(), post.getCaption(),
                media.getId(), mediaUrls.urlsFor(media.getVariants()),
                counters.likeCount(postId), post.getCreatedAt(), score);
    }
}
