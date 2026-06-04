package com.ankitsriv89.chat.service;

import com.ankitsriv89.chat.domain.Conversation;
import com.ankitsriv89.chat.repository.ConversationRepository;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;

@Service
public class ConversationService {

    private final ConversationRepository repo;
    private final IdGenerator ids;

    public ConversationService(ConversationRepository repo, IdGenerator ids) {
        this.repo = repo;
        this.ids = ids;
    }

    @Transactional
    public Conversation getOrCreate(String userA, String userB) {
        return repo.findByParticipants(userA, userB)
            .orElseGet(() -> repo.save(new Conversation(ids.nextId(), userA, userB)));
    }

    @Transactional(readOnly = true)
    public Conversation getById(Long id) {
        return repo.findById(id)
            .orElseThrow(() -> new IllegalArgumentException("conversation not found: " + id));
    }

    @Transactional(readOnly = true)
    public List<Conversation> listForUser(String userId) {
        return repo.findByParticipant(userId);
    }

    @Transactional
    public long nextSeq(Long conversationId) {
        Conversation c = getById(conversationId);
        long seq = c.incrementSeq();
        repo.save(c);
        return seq;
    }
}
