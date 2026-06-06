package com.ankitsriv89.instagram;

import com.ankitsriv89.instagram.dto.PostCreatedEvent;
import com.ankitsriv89.instagram.service.FanoutService;
import com.ankitsriv89.instagram.service.FollowService;
import com.ankitsriv89.instagram.service.RankingService;
import com.ankitsriv89.instagram.store.TimelineStore;
import org.junit.jupiter.api.Test;

import java.time.Instant;
import java.util.List;

import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * Milestone 3 acceptance — hybrid fanout: push for normal authors, skip for
 * celebrities above the threshold.
 */
class FanoutServiceTest {

    private final FollowService follows = mock(FollowService.class);
    private final TimelineStore timelines = mock(TimelineStore.class);
    private final RankingService ranking = new RankingService(12.0);
    // celebrity threshold = 1000 for the test
    private final FanoutService fanout = new FanoutService(follows, timelines, ranking, 1000);

    @Test
    void normalAuthor_isFannedOut() {
        when(follows.followerCount(7L)).thenReturn(3L);
        when(follows.followersOf(7L)).thenReturn(List.of(1L, 2L, 3L));

        fanout.onPostCreated(new PostCreatedEvent(100L, 7L, Instant.now()));

        verify(timelines).pushMany(eq(List.of(1L, 2L, 3L)), eq(100L), anyDouble());
    }

    @Test
    void celebrity_isSkipped() {
        when(follows.followerCount(7L)).thenReturn(50_000L);

        fanout.onPostCreated(new PostCreatedEvent(100L, 7L, Instant.now()));

        verify(timelines, never()).pushMany(anyList(), anyLong(), anyDouble());
        verify(follows, never()).followersOf(anyLong());
    }
}
