package com.ankitsriv89.ecommerce.service;

import com.ankitsriv89.ecommerce.domain.Order;
import com.ankitsriv89.ecommerce.domain.OrderStatus;
import com.ankitsriv89.ecommerce.repository.OrderRepository;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;
import java.util.Optional;

@Service
public class OrderService {

    private static final Logger log = LoggerFactory.getLogger(OrderService.class);

    private final OrderRepository orderRepo;

    public OrderService(OrderRepository orderRepo) {
        this.orderRepo = orderRepo;
    }

    @Transactional(readOnly = true)
    public Optional<Order> getOrder(String id) {
        return orderRepo.findById(id);
    }

    @Transactional(readOnly = true)
    public List<Order> getOrdersByUser(String userId) {
        return orderRepo.findByUserId(userId);
    }

    @Transactional
    public Order transition(String orderId, OrderStatus newStatus) {
        Order order = orderRepo.findById(orderId)
                .orElseThrow(() -> new IllegalArgumentException("Order not found: " + orderId));
        log.info("order.transition id={} {} -> {}", orderId, order.getStatus(), newStatus);
        order.setStatus(newStatus);
        return orderRepo.save(order);
    }

    @Transactional(readOnly = true)
    public List<Order> listAll() {
        return orderRepo.findAll();
    }
}
