package com.ankitsriv89.whatsapp.api;

import com.ankitsriv89.whatsapp.dto.MessageResponse;
import com.ankitsriv89.whatsapp.dto.SendMessageRequest;
import com.ankitsriv89.whatsapp.service.MessageService;
import jakarta.validation.Valid;
import org.springframework.format.annotation.DateTimeFormat;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.Authentication;
import org.springframework.web.bind.annotation.*;

import java.time.Instant;
import java.util.List;

@RestController
@RequestMapping("/v1/messages")
public class MessageController {

    private final MessageService messageService;

    public MessageController(MessageService messageService) { this.messageService = messageService; }

    @PostMapping
    public ResponseEntity<MessageResponse> send(
            Authentication auth,
            @Valid @RequestBody SendMessageRequest req) {
        return ResponseEntity.ok(messageService.send(auth.getName(), req));
    }

    /** Sync: GET /v1/messages/sync?chatId=dm:1:2&since=<ISO-8601>
     *  Caller must be a participant in the chat; returns 403 otherwise. */
    @GetMapping("/sync")
    public ResponseEntity<List<MessageResponse>> sync(
            Authentication auth,
            @RequestParam String chatId,
            @RequestParam(required = false) @DateTimeFormat(iso = DateTimeFormat.ISO.DATE_TIME) Instant since) {
        return ResponseEntity.ok(messageService.sync(auth.getName(), chatId, since));
    }
}
