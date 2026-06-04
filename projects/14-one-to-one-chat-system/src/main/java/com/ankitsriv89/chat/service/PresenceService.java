package com.ankitsriv89.chat.service;

import com.ankitsriv89.chat.dto.PresenceDto;
import com.ankitsriv89.chat.store.PresenceStore;
import org.springframework.stereotype.Service;

import java.util.List;
import java.util.Map;

@Service
public class PresenceService {

    private final PresenceStore store;

    public PresenceService(PresenceStore store) {
        this.store = store;
    }

    public void heartbeat(String userId) {
        store.heartbeat(userId);
    }

    public void markOffline(String userId) {
        store.offline(userId);
    }

    public PresenceDto get(String userId) {
        boolean online = store.isOnline(userId);
        Long lastSeen = online ? System.currentTimeMillis() : store.lastSeenEpochMs(userId);
        return new PresenceDto(userId, online, lastSeen);
    }

    public List<PresenceDto> bulkGet(List<String> userIds) {
        Map<String, Boolean> statuses = store.bulkStatus(userIds);
        return userIds.stream().map(id -> {
            boolean online = Boolean.TRUE.equals(statuses.get(id));
            Long lastSeen = online ? System.currentTimeMillis() : store.lastSeenEpochMs(id);
            return new PresenceDto(id, online, lastSeen);
        }).toList();
    }
}
