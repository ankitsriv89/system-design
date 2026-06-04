package com.ankitsriv89.newsfeed.service;

import com.ankitsriv89.newsfeed.domain.Post;
import com.ankitsriv89.newsfeed.dto.PostCreatedEvent;
import com.ankitsriv89.newsfeed.metrics.FeedMetrics;
import com.ankitsriv89.newsfeed.repository.PostRepository;
import com.ankitsriv89.newsfeed.store.EventPublisher;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;

import java.time.Instant;
import java.util.Optional;

/**
 * Write path for posts. A post is committed to PostgreSQL first; the
 * post.created event is published to Kafka only <em>after</em> the DB
 * transaction commits. This ordering guarantees we never fan out a post that
 * wasn't durably stored — the source of truth leads, the async pipeline follows.
 */
@Service
public class PostService {

    private final PostRepository posts;
    private final EventPublisher events;
    private final FeedMetrics metrics;

    public PostService(PostRepository posts, EventPublisher events, FeedMetrics metrics) {
        this.posts = posts;
        this.events = events;
        this.metrics = metrics;
    }

    @Transactional
    public Post create(String authorId, String body) {
        Post saved = posts.save(new Post(authorId, body, Instant.now()));
        PostCreatedEvent event = new PostCreatedEvent(
                saved.getId(), saved.getAuthorId(), saved.getBody(), saved.getCreatedAt());

        // Register an after-commit callback so the event fires only once the row
        // is durable. If the transaction rolls back, no event is published.
        TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
            @Override
            public void afterCommit() {
                events.publishPostCreated(event);
            }
        });

        metrics.recordPostCreated();
        return saved;
    }

    @Transactional
    public void delete(String requesterId, long postId) {
        Optional<Post> found = posts.findById(postId);
        if (found.isEmpty()) {
            return;
        }
        Post post = found.get();
        if (!post.getAuthorId().equals(requesterId)) {
            throw new SecurityException("only the author can delete this post");
        }
        // Soft delete: the row stays for audit, but the read path filters it out
        // and the feed assembler drops it. This is how we handle the
        // "deleted post remains in feed" failure mode — deletes are honored
        // lazily at read time even if a stale timeline entry lingers in Redis.
        post.setDeleted(true);
        posts.save(post);
    }

    public Optional<Post> find(long postId) {
        return posts.findById(postId);
    }
}
