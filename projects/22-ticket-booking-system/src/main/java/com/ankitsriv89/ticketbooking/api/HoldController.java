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
    public ResponseEntity<Hold> createHold(@RequestBody Map<String, String> body) {
        String seatId = body.get("seatId");
        String userId = body.get("userId");
        if (seatId == null || userId == null) {
            return ResponseEntity.badRequest().build();
        }
        return ResponseEntity.status(201).body(holdService.createHold(seatId, userId));
    }

    @GetMapping("/{id}")
    public Hold getHold(@PathVariable String id) {
        return holdService.getHold(id);
    }
}
