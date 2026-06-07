package com.ankitsriv89.ecommerce.store;

import com.ankitsriv89.ecommerce.domain.Order;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.stereotype.Component;

@Component
public class EventPublisher {

    private static final Logger log = LoggerFactory.getLogger(EventPublisher.class);

    static final String TOPIC_ORDERS = "ecommerce.orders";

    private final KafkaTemplate<String, OrderEvent> kafka;

    public EventPublisher(KafkaTemplate<String, OrderEvent> kafka) {
        this.kafka = kafka;
    }

    public void publishOrderCreated(Order order) {
        OrderEvent event = new OrderEvent(order.getId(), order.getUserId(),
                order.getStatus(), order.getTotal());
        kafka.send(TOPIC_ORDERS, order.getId(), event)
             .whenComplete((r, ex) -> {
                 if (ex != null) {
                     log.error("kafka.send.failed topic={} orderId={}", TOPIC_ORDERS, order.getId(), ex);
                 } else {
                     log.info("kafka.sent topic={} orderId={} offset={}",
                             TOPIC_ORDERS, order.getId(), r.getRecordMetadata().offset());
                 }
             });
    }
}
