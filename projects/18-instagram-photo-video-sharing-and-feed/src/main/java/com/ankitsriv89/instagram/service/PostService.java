package com.ankitsriv89.instagram.service;

import com.ankitsriv89.instagram.domain.Media;
import com.ankitsriv89.instagram.domain.Post;
import com.ankitsriv89.instagram.dto.PostCreatedEvent;
import com.ankitsriv89.instagram.repository.PostRepository;
import com.ankitsriv89.instagram.store.EventPublisher;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/**
 * Post creation. A post references a Media the caller owns. We allow posting a
 * PENDING media (variants may still be in flight) so the write path isn't
 * blocked on processing — the feed shows the original until variants land. The
 * post.created event drives fanout.
 */
@Service
public class PostService {

    private static final Logger log = LoggerFactory.getLogger(PostService.class);

    private final PostRepository postRepository;
    private final MediaService mediaService;
    private final EventPublisher eventPublisher;

    public PostService(PostRepository postRepository,
                       MediaService mediaService,
                       EventPublisher eventPublisher) {
        this.postRepository = postRepository;
        this.mediaService = mediaService;
        this.eventPublisher = eventPublisher;
    }

    @Transactional
    public Post createPost(Long userId, Long mediaId, String caption) {
        Media media = mediaService.get(mediaId);
        if (!media.getOwnerId().equals(userId)) {
            throw new SecurityException("media " + mediaId + " not owned by user " + userId);
        }
        Post post = postRepository.save(new Post(userId, mediaId, caption));
        eventPublisher.publishPostCreated(
                new PostCreatedEvent(post.getId(), userId, post.getCreatedAt()));
        log.debug("created post_id={} user={} media={}", post.getId(), userId, mediaId);
        return post;
    }

    @Transactional(readOnly = true)
    public Post get(Long postId) {
        return postRepository.findById(postId)
                .orElseThrow(() -> new IllegalArgumentException("post not found: " + postId));
    }
}
