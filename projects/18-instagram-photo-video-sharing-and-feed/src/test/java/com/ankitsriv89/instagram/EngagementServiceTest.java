package com.ankitsriv89.instagram;

import com.ankitsriv89.instagram.repository.EngagementRepository;
import com.ankitsriv89.instagram.service.EngagementService;
import com.ankitsriv89.instagram.store.CounterStore;
import com.ankitsriv89.instagram.store.EventPublisher;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * Milestone 3 acceptance — like/unlike idempotency and counter behavior.
 */
@ExtendWith(MockitoExtension.class)
class EngagementServiceTest {

    @Mock EngagementRepository engagementRepository;
    @Mock CounterStore counterStore;
    @Mock EventPublisher eventPublisher;
    @InjectMocks EngagementService engagementService;

    @Test
    void firstLike_incrementsAndEmits() {
        when(engagementRepository.existsByPostIdAndUserIdAndType(1L, 42L, "LIKE")).thenReturn(false);
        when(counterStore.incrementLikes(1L)).thenReturn(1L);

        long count = engagementService.like(1L, 42L);

        assertThat(count).isEqualTo(1L);
        verify(engagementRepository).save(any());
        verify(eventPublisher).publishPostLiked(any());
    }

    @Test
    void duplicateLike_isNoOp() {
        when(engagementRepository.existsByPostIdAndUserIdAndType(1L, 42L, "LIKE")).thenReturn(true);
        when(counterStore.likeCount(1L)).thenReturn(5L);

        long count = engagementService.like(1L, 42L);

        assertThat(count).isEqualTo(5L);
        verify(engagementRepository, never()).save(any());
        verify(counterStore, never()).incrementLikes(anyLong());
        verifyNoInteractions(eventPublisher);
    }

    @Test
    void unlike_whenNotLiked_isNoOp() {
        when(engagementRepository.existsByPostIdAndUserIdAndType(1L, 42L, "LIKE")).thenReturn(false);
        when(counterStore.likeCount(1L)).thenReturn(0L);

        long count = engagementService.unlike(1L, 42L);

        assertThat(count).isZero();
        verify(counterStore, never()).decrementLikes(anyLong());
        verifyNoInteractions(eventPublisher);
    }
}
