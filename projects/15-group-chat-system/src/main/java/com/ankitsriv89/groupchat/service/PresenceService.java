package com.ankitsriv89.groupchat.service;

import com.ankitsriv89.groupchat.store.PresenceStore;
import org.springframework.stereotype.Service;

import java.util.List;
import java.util.Set;

@Service
public class PresenceService {

    private final PresenceStore store;

    public PresenceService(PresenceStore store) {
        this.store = store;
    }

    public void online(String userId) { store.setOnline(userId); }
    public void offline(String userId) { store.setOffline(userId); }
    public boolean isOnline(String userId) { return store.isOnline(userId); }
    public Set<String> onlineAmong(List<String> userIds) { return store.onlineAmong(userIds); }
}
