package com.ankitsriv89.groupchat.controller;

import com.ankitsriv89.groupchat.domain.Group;
import com.ankitsriv89.groupchat.domain.GroupMember;
import com.ankitsriv89.groupchat.service.GroupService;
import com.ankitsriv89.groupchat.service.PresenceService;
import org.springframework.http.HttpStatus;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.web.bind.annotation.*;

import java.util.List;
import java.util.Map;
import java.util.Set;

@RestController
@RequestMapping("/api/groups")
public class GroupController {

    private final GroupService groupService;
    private final PresenceService presenceService;

    public GroupController(GroupService groupService, PresenceService presenceService) {
        this.groupService = groupService;
        this.presenceService = presenceService;
    }

    @PostMapping
    @ResponseStatus(HttpStatus.CREATED)
    public Group create(@AuthenticationPrincipal String userId,
                        @RequestBody Map<String, String> body) {
        return groupService.create(body.get("name"), userId);
    }

    @GetMapping
    public List<Group> myGroups(@AuthenticationPrincipal String userId) {
        return groupService.groupsForUser(userId);
    }

    @GetMapping("/{groupId}")
    public Group get(@PathVariable Long groupId, @AuthenticationPrincipal String userId) {
        groupService.requireMember(groupId, userId);
        return groupService.get(groupId);
    }

    @GetMapping("/{groupId}/members")
    public List<GroupMember> members(@PathVariable Long groupId,
                                     @AuthenticationPrincipal String userId) {
        groupService.requireMember(groupId, userId);
        List<GroupMember> members = groupService.members(groupId);
        Set<String> online = presenceService.onlineAmong(members.stream().map(GroupMember::getUserId).toList());
        return members;
    }

    @PostMapping("/{groupId}/members")
    @ResponseStatus(HttpStatus.CREATED)
    public void addMember(@PathVariable Long groupId,
                          @AuthenticationPrincipal String userId,
                          @RequestBody Map<String, String> body) {
        groupService.addMember(groupId, userId, body.get("userId"));
    }

    @DeleteMapping("/{groupId}/members/{targetUserId}")
    @ResponseStatus(HttpStatus.NO_CONTENT)
    public void removeMember(@PathVariable Long groupId,
                             @AuthenticationPrincipal String userId,
                             @PathVariable String targetUserId) {
        groupService.removeMember(groupId, userId, targetUserId);
    }
}
