package com.ankitsriv89.chat.controller;

import com.ankitsriv89.chat.dto.ConversationDto;
import com.ankitsriv89.chat.dto.MessageDto;
import com.ankitsriv89.chat.dto.SendMessageRequest;
import com.ankitsriv89.chat.service.ConversationService;
import com.ankitsriv89.chat.service.MessageService;
import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.*;

import java.security.Principal;
import java.util.List;

@RestController
@RequestMapping("/api/v1/conversations")
public class ConversationController {

    private final ConversationService convService;
    private final MessageService msgService;

    public ConversationController(ConversationService convService, MessageService msgService) {
        this.convService = convService;
        this.msgService = msgService;
    }

    @GetMapping
    public List<ConversationDto> list(Principal principal) {
        return convService.listForUser(principal.getName())
            .stream().map(ConversationDto::from).toList();
    }

    @PostMapping
    @ResponseStatus(HttpStatus.CREATED)
    public ConversationDto create(@RequestParam String recipientId, Principal principal) {
        return ConversationDto.from(convService.getOrCreate(principal.getName(), recipientId));
    }

    @PostMapping("/{conversationId}/messages")
    @ResponseStatus(HttpStatus.CREATED)
    public MessageDto sendMessage(@PathVariable Long conversationId,
                                  @RequestBody SendMessageRequest req,
                                  Principal principal) {
        // Verify caller is a participant before sending
        var conv = convService.getById(conversationId);
        if (!conv.hasParticipant(principal.getName())) {
            throw new org.springframework.security.access.AccessDeniedException("not a participant");
        }
        String recipientId = conv.otherParticipant(principal.getName());
        return msgService.send(principal.getName(), recipientId, req.body());
    }

    @GetMapping("/{conversationId}/messages")
    public List<MessageDto> history(@PathVariable Long conversationId,
                                    @RequestParam(required = false) Long before,
                                    @RequestParam(defaultValue = "50") int limit,
                                    Principal principal) {
        var conv = convService.getById(conversationId);
        if (!conv.hasParticipant(principal.getName())) {
            throw new org.springframework.security.access.AccessDeniedException("not a participant");
        }
        return msgService.history(conversationId, before, Math.min(limit, 100));
    }
}
