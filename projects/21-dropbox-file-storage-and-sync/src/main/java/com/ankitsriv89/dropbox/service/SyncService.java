package com.ankitsriv89.dropbox.service;

import com.ankitsriv89.dropbox.domain.ChangeEvent;
import com.ankitsriv89.dropbox.domain.SyncCursor;
import com.ankitsriv89.dropbox.repository.SyncCursorRepository;
import lombok.RequiredArgsConstructor;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.Instant;
import java.util.List;

@Service
@RequiredArgsConstructor
public class SyncService {
    private final SyncCursorRepository cursorRepo;
    private final FileService fileService;
    private final StringRedisTemplate redis;

    private static final String CURSOR_KEY = "dropbox:cursor:";

    @Transactional
    public SyncResult poll(String ownerId, String deviceId) {
        long lastEventId = getCursor(ownerId, deviceId);
        List<ChangeEvent> events = fileService.syncFeed(ownerId, lastEventId);
        if (!events.isEmpty()) {
            long newCursor = events.getLast().getId();
            saveCursor(ownerId, deviceId, newCursor);
        }
        return new SyncResult(events, lastEventId);
    }

    private long getCursor(String ownerId, String deviceId) {
        String cached = redis.opsForValue().get(CURSOR_KEY + deviceId);
        if (cached != null) return Long.parseLong(cached);
        return cursorRepo.findById(deviceId)
                .map(SyncCursor::getLastEventId)
                .orElse(0L);
    }

    private void saveCursor(String ownerId, String deviceId, long eventId) {
        redis.opsForValue().set(CURSOR_KEY + deviceId, String.valueOf(eventId));
        SyncCursor cursor = cursorRepo.findById(deviceId).orElseGet(() -> {
            SyncCursor c = new SyncCursor();
            c.setDeviceId(deviceId);
            c.setOwnerId(ownerId);
            return c;
        });
        cursor.setLastEventId(eventId);
        cursor.setUpdatedAt(Instant.now());
        cursorRepo.save(cursor);
    }

    public record SyncResult(List<ChangeEvent> events, long previousCursor) {}
}
