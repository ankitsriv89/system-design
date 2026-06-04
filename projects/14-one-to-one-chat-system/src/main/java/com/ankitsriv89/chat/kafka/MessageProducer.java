package com.ankitsriv89.chat.kafka;

import com.ankitsriv89.chat.dto.KafkaMessageEvent;
import com.ankitsriv89.chat.dto.ReceiptEvent;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.stereotype.Component;

@Component
public class MessageProducer {

    private final KafkaTemplate<String, Object> kafka;
    private final String messagesTopic;
    private final String receiptsTopic;

    public MessageProducer(KafkaTemplate<String, Object> kafka,
                           @Value("${chat.kafka.topic.messages}") String messagesTopic,
                           @Value("${chat.kafka.topic.receipts}") String receiptsTopic) {
        this.kafka = kafka;
        this.messagesTopic = messagesTopic;
        this.receiptsTopic = receiptsTopic;
    }

    // Partition by recipientId so messages for the same recipient always land on the same partition
    public void publishMessage(KafkaMessageEvent event) {
        kafka.send(messagesTopic, event.recipientId(), event);
    }

    public void publishReceipt(ReceiptEvent event) {
        kafka.send(receiptsTopic, event.userId(), event);
    }
}
