package com.ankitsriv89.chat.controller;

import com.ankitsriv89.chat.dto.PresenceDto;
import com.ankitsriv89.chat.service.PresenceService;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/api/v1/presence")
public class PresenceController {

    private final PresenceService presence;

    public PresenceController(PresenceService presence) {
        this.presence = presence;
    }

    @GetMapping
    public List<PresenceDto> bulk(@RequestParam List<String> users) {
        return presence.bulkGet(users);
    }

    @GetMapping("/{userId}")
    public PresenceDto single(@PathVariable String userId) {
        return presence.get(userId);
    }
}
