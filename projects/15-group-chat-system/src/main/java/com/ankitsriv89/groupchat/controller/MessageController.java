package com.ankitsriv89.groupchat.controller;

import com.ankitsriv89.groupchat.domain.Message;
import com.ankitsriv89.groupchat.dto.SendMessageRequest;
import com.ankitsriv89.groupchat.service.MessageService;
import org.springframework.http.HttpStatus;
import org.springframework.messaging.handler.annotation.MessageMapping;
import org.springframework.messaging.handler.annotation.Payload;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.web.bind.annotation.*;

import java.security.Principal;
import java.util.List;

@RestController
public class MessageController {

    private final MessageService messageService;

    public MessageController(MessageService messageService) {
        this.messageService = messageService;
    }

    @PostMapping("/api/groups/{groupId}/messages")
    @ResponseStatus(HttpStatus.CREATED)
    public Message send(@PathVariable Long groupId,
                        @AuthenticationPrincipal String userId,
                        @RequestBody SendMessageRequest req) {
        return messageService.send(groupId, userId, req.getBody());
    }

    @GetMapping("/api/groups/{groupId}/messages")
    public List<Message> history(@PathVariable Long groupId,
                                 @AuthenticationPrincipal String userId,
                                 @RequestParam(required = false) Long before) {
        if (before != null) return messageService.before(groupId, userId, before);
        return messageService.recent(groupId, userId);
    }

    @MessageMapping("/chat.send")
    public void wsSend(@Payload SendMessageRequest req, Principal principal) {
        if (principal == null) return;
        messageService.send(req.getGroupId(), principal.getName(), req.getBody());
    }
}
