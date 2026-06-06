package com.ankitsriv89.whatsapp.api;

import com.ankitsriv89.whatsapp.dto.GroupCreateRequest;
import com.ankitsriv89.whatsapp.dto.GroupResponse;
import com.ankitsriv89.whatsapp.service.GroupService;
import jakarta.validation.Valid;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.Authentication;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/v1/groups")
public class GroupController {

    private final GroupService groupService;

    public GroupController(GroupService groupService) { this.groupService = groupService; }

    @PostMapping
    public ResponseEntity<GroupResponse> create(
            Authentication auth,
            @Valid @RequestBody GroupCreateRequest req) {
        return ResponseEntity.ok(groupService.create(auth.getName(), req));
    }

    @PostMapping("/{groupId}/members")
    public ResponseEntity<Void> addMember(
            Authentication auth,
            @PathVariable Long groupId,
            @RequestParam Long userId) {
        groupService.addMember(groupId, userId, auth.getName());
        return ResponseEntity.noContent().build();
    }

    @GetMapping
    public ResponseEntity<List<GroupResponse>> list(Authentication auth) {
        return ResponseEntity.ok(groupService.listForUser(auth.getName()));
    }
}
