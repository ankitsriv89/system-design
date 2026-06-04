package com.ankitsriv89.groupchat.controller;

import com.ankitsriv89.groupchat.service.PresenceService;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import java.util.List;
import java.util.Map;
import java.util.Set;

@RestController
@RequestMapping("/api/presence")
public class PresenceController {

    private final PresenceService presenceService;

    public PresenceController(PresenceService presenceService) {
        this.presenceService = presenceService;
    }

    @GetMapping
    public Map<String, Object> presence(@AuthenticationPrincipal String userId,
                                        @RequestParam List<String> userIds) {
        Set<String> online = presenceService.onlineAmong(userIds);
        return Map.of("online", online);
    }
}
