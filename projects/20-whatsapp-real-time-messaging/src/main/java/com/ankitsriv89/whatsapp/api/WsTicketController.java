package com.ankitsriv89.whatsapp.api;

import com.ankitsriv89.whatsapp.store.WsTicketStore;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.Authentication;
import org.springframework.web.bind.annotation.*;

import java.util.Map;

/**
 * Issues a short-lived ticket for the WebSocket upgrade so the JWT never
 * appears in the query string (and therefore not in access logs or history).
 *
 * Flow:
 *   1. Client calls POST /v1/ws-ticket?deviceId=N  (with Authorization: Bearer <jwt>)
 *   2. Server stores ticket → (username, deviceId) in Redis with 30 s TTL.
 *   3. Client opens  ws://.../ws/v1/session?ticket=<ticket>
 *   4. SessionHandler redeems the ticket once; subsequent reuse is rejected.
 */
@RestController
@RequestMapping("/v1/ws-ticket")
public class WsTicketController {

    private final WsTicketStore ticketStore;

    public WsTicketController(WsTicketStore ticketStore) { this.ticketStore = ticketStore; }

    @PostMapping
    public ResponseEntity<Map<String, String>> issue(Authentication auth, @RequestParam Long deviceId) {
        String ticket = ticketStore.issue(auth.getName(), deviceId);
        return ResponseEntity.ok(Map.of("ticket", ticket));
    }
}
