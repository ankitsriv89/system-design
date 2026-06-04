package com.ankitsriv89.chat.service;

import com.ankitsriv89.chat.domain.Conversation;
import com.ankitsriv89.chat.domain.Message;
import com.ankitsriv89.chat.dto.KafkaMessageEvent;
import com.ankitsriv89.chat.dto.MessageDto;
import com.ankitsriv89.chat.kafka.MessageProducer;
import com.ankitsriv89.chat.repository.MessageRepository;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;

@Service
public class MessageService {

    private final MessageRepository msgRepo;
    private final ConversationService convService;
    private final IdGenerator ids;
    private final MessageProducer producer;

    public MessageService(MessageRepository msgRepo, ConversationService convService,
                          IdGenerator ids, MessageProducer producer) {
        this.msgRepo = msgRepo;
        this.convService = convService;
        this.ids = ids;
        this.producer = producer;
    }

    @Transactional
    public MessageDto send(String senderId, String recipientId, String body) {
        Conversation conv = convService.getOrCreate(senderId, recipientId);
        long seq = convService.nextSeq(conv.getId());
        Message msg = msgRepo.save(new Message(ids.nextId(), conv.getId(), senderId, body, seq));
        MessageDto dto = MessageDto.from(msg);
        // Publish to Kafka so the consumer can push to recipient over WebSocket
        // (or buffer for offline delivery)
        producer.publishMessage(new KafkaMessageEvent(dto, recipientId));
        return dto;
    }

    @Transactional(readOnly = true)
    public List<MessageDto> history(Long conversationId, Long beforeSeq, int limit) {
        return msgRepo.findPage(conversationId, beforeSeq, limit)
            .stream().map(MessageDto::from).toList();
    }

    @Transactional
    public void markDelivered(Long messageId) {
        msgRepo.findById(messageId).ifPresent(m -> {
            m.markDelivered();
            msgRepo.save(m);
        });
    }

    @Transactional
    public void markRead(Long messageId) {
        msgRepo.findById(messageId).ifPresent(m -> {
            m.markRead();
            msgRepo.save(m);
        });
    }
}
