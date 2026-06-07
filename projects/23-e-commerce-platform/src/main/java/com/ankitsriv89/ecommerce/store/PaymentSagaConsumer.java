package com.ankitsriv89.ecommerce.store;

import com.ankitsriv89.ecommerce.domain.OrderStatus;
import com.ankitsriv89.ecommerce.service.InventoryService;
import com.ankitsriv89.ecommerce.service.OrderService;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.stereotype.Component;

// Simulates the async payment step of the order saga.
// In a real system, this would call a payment gateway; here it auto-approves
// all orders so the happy path is demonstrable without external dependencies.
@Component
public class PaymentSagaConsumer {

    private static final Logger log = LoggerFactory.getLogger(PaymentSagaConsumer.class);

    private final OrderService orderService;
    private final InventoryService inventoryService;

    public PaymentSagaConsumer(OrderService orderService, InventoryService inventoryService) {
        this.orderService = orderService;
        this.inventoryService = inventoryService;
    }

    @KafkaListener(topics = EventPublisher.TOPIC_ORDERS,
                   groupId = "payment-saga",
                   containerFactory = "kafkaListenerContainerFactory")
    public void onOrderEvent(OrderEvent event) {
        if (event.getStatus() != OrderStatus.INVENTORY_RESERVED) {
            return; // Only act on orders awaiting payment.
        }

        log.info("saga.payment orderId={} total={}", event.getOrderId(), event.getTotal());

        // Mock payment: always succeeds for demo.
        try {
            orderService.transition(event.getOrderId(), OrderStatus.PAYMENT_AUTHORIZED);
            orderService.transition(event.getOrderId(), OrderStatus.CONFIRMED);
            log.info("saga.payment_authorized orderId={}", event.getOrderId());
        } catch (Exception e) {
            log.error("saga.payment_failed orderId={}", event.getOrderId(), e);
            inventoryService.release(event.getOrderId());
            orderService.transition(event.getOrderId(), OrderStatus.PAYMENT_FAILED);
        }
    }
}
