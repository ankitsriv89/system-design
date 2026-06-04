package com.ankitsriv89.chat;

import com.ankitsriv89.chat.domain.Conversation;
import com.ankitsriv89.chat.repository.ConversationRepository;
import com.ankitsriv89.chat.service.ConversationService;
import com.ankitsriv89.chat.service.IdGenerator;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

import java.util.Optional;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

class ConversationServiceTest {

    private ConversationRepository repo;
    private ConversationService service;
    private IdGenerator ids;

    @BeforeEach
    void setUp() {
        repo = Mockito.mock(ConversationRepository.class);
        ids = Mockito.mock(IdGenerator.class);
        service = new ConversationService(repo, ids);
    }

    @Test
    void getOrCreate_returnsExisting() {
        Conversation existing = new Conversation(1L, "alice", "bob");
        when(repo.findByParticipants("alice", "bob")).thenReturn(Optional.of(existing));

        Conversation result = service.getOrCreate("alice", "bob");

        assertSame(existing, result);
        verify(repo, never()).save(any());
    }

    @Test
    void getOrCreate_createsNew() {
        when(repo.findByParticipants(any(), any())).thenReturn(Optional.empty());
        when(ids.nextId()).thenReturn(42L);
        Conversation saved = new Conversation(42L, "alice", "bob");
        when(repo.save(any())).thenReturn(saved);

        Conversation result = service.getOrCreate("alice", "bob");

        verify(repo).save(any(Conversation.class));
        assertNotNull(result);
    }

    @Test
    void conversation_canonicalOrder() {
        // userB < userA alphabetically — constructor must swap them
        Conversation c = new Conversation(1L, "zara", "alice");
        assertEquals("alice", c.getUserA());
        assertEquals("zara", c.getUserB());
    }

    @Test
    void conversation_hasParticipant() {
        Conversation c = new Conversation(1L, "alice", "bob");
        assertTrue(c.hasParticipant("alice"));
        assertTrue(c.hasParticipant("bob"));
        assertFalse(c.hasParticipant("charlie"));
    }

    @Test
    void conversation_otherParticipant() {
        Conversation c = new Conversation(1L, "alice", "bob");
        assertEquals("bob", c.otherParticipant("alice"));
        assertEquals("alice", c.otherParticipant("bob"));
    }

    @Test
    void conversation_incrementSeq() {
        Conversation c = new Conversation(1L, "alice", "bob");
        assertEquals(1L, c.incrementSeq());
        assertEquals(2L, c.incrementSeq());
    }
}
