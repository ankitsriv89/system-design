package com.ankitsriv89.ticketbooking.api;

import com.ankitsriv89.ticketbooking.domain.Hold;
import com.ankitsriv89.ticketbooking.service.HoldService;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.Map;

@RestController
@RequestMapping("/v1/holds")
public class HoldController {

    private final HoldService holdService;

    public HoldController(HoldService holdService) {
        this.holdService = holdService;
    }

    @PostMapping
    public ResponseEntity<Hold> createHold(
            @RequestHeader("X-User-Id") String callerId,
            @RequestBody Map<String, String> body) {
        String seatId = body.get("seatId");
        if (seatId == null) {
            return ResponseEntity.badRequest().build();
        }
        return ResponseEntity.status(201).body(holdService.createHold(seatId, callerId));
    }

    @GetMapping("/{id}")
    public ResponseEntity<Hold> getHold(
            @RequestHeader("X-User-Id") String callerId,
            @PathVariable String id) {
        Hold hold = holdService.getHold(id);
        if (!hold.getUserId().equals(callerId)) {
            return ResponseEntity.status(403).build();
        }
        return ResponseEntity.ok(hold);
    }
}
