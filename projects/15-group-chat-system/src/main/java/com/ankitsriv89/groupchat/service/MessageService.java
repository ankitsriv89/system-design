package com.ankitsriv89.groupchat.service;

import com.ankitsriv89.groupchat.domain.Message;
import com.ankitsriv89.groupchat.dto.KafkaMessageEvent;
import com.ankitsriv89.groupchat.kafka.MessageProducer;
import com.ankitsriv89.groupchat.repository.MessageRepository;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;

@Service
public class MessageService {

    private final MessageRepository messageRepo;
    private final GroupService groupService;
    private final MessageProducer producer;
    private final IdGenerator ids;

    public MessageService(MessageRepository messageRepo, GroupService groupService,
                          MessageProducer producer, IdGenerator ids) {
        this.messageRepo = messageRepo;
        this.groupService = groupService;
        this.producer = producer;
        this.ids = ids;
    }

    @Transactional
    public Message send(Long groupId, String senderId, String body) {
        if (body == null || body.isBlank()) throw new IllegalArgumentException("body required");
        groupService.requireMember(groupId, senderId);

        long seq = messageRepo.findMaxSeqByGroupId(groupId).orElse(0L) + 1;
        Message msg = new Message(ids.next(), groupId, senderId, body.trim(), seq);
        messageRepo.save(msg);

        producer.publish(new KafkaMessageEvent(msg.getId(), groupId, senderId, body, seq));
        return msg;
    }

    public List<Message> recent(Long groupId, String userId) {
        groupService.requireMember(groupId, userId);
        return messageRepo.findTop50ByGroupIdOrderBySeqDesc(groupId);
    }

    public List<Message> before(Long groupId, String userId, Long beforeSeq) {
        groupService.requireMember(groupId, userId);
        return messageRepo.findByGroupIdAndSeqLessThanOrderBySeqDesc(groupId, beforeSeq);
    }
}
