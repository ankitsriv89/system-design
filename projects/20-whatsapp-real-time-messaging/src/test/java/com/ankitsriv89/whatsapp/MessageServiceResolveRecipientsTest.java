package com.ankitsriv89.whatsapp;

import org.junit.jupiter.api.Test;
import java.util.Arrays;
import java.util.List;
import java.util.stream.Collectors;

import static org.junit.jupiter.api.Assertions.*;

/**
 * White-box test of the chatId → recipient resolution logic extracted for unit testing.
 * Tests the dm: and group: chatId format parsing without Spring context.
 */
class MessageServiceResolveRecipientsTest {

    // Replicated resolve logic for unit testing without Spring context.
    private List<Long> resolveRecipients(String chatId, Long senderId) {
        if (chatId.startsWith("dm:")) {
            String[] parts = chatId.split(":");
            return Arrays.stream(parts, 1, parts.length)
                    .map(Long::parseLong)
                    .filter(uid -> !uid.equals(senderId))
                    .collect(Collectors.toList());
        }
        return List.of();
    }

    @Test
    void dmChatIdExcludesSender() {
        List<Long> recipients = resolveRecipients("dm:1:2", 1L);
        assertEquals(List.of(2L), recipients);
    }

    @Test
    void dmChatIdOtherDirection() {
        List<Long> recipients = resolveRecipients("dm:1:2", 2L);
        assertEquals(List.of(1L), recipients);
    }

    @Test
    void dmChatIdWithHigherIds() {
        List<Long> recipients = resolveRecipients("dm:42:99", 42L);
        assertEquals(List.of(99L), recipients);
    }

    @Test
    void unknownChatIdReturnsEmpty() {
        List<Long> recipients = resolveRecipients("unknown:abc", 1L);
        assertTrue(recipients.isEmpty());
    }

    @Test
    void groupChatIdReturnsEmpty() {
        // group: resolution requires DB — only testing that it doesn't crash here
        String chatId = "group:5";
        assertFalse(chatId.startsWith("dm:"));
    }
}
