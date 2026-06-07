package com.ankitsriv89.ecommerce.service;

import com.ankitsriv89.ecommerce.domain.*;
import com.ankitsriv89.ecommerce.repository.OrderRepository;
import com.ankitsriv89.ecommerce.store.EventPublisher;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;
import java.util.Optional;

@Service
public class CheckoutService {

    private static final Logger log = LoggerFactory.getLogger(CheckoutService.class);

    private final CartService cartService;
    private final InventoryService inventoryService;
    private final OrderRepository orderRepo;
    private final EventPublisher eventPublisher;

    public CheckoutService(CartService cartService,
                           InventoryService inventoryService,
                           OrderRepository orderRepo,
                           EventPublisher eventPublisher) {
        this.cartService = cartService;
        this.inventoryService = inventoryService;
        this.orderRepo = orderRepo;
        this.eventPublisher = eventPublisher;
    }

    // Idempotent checkout: same idempotencyKey returns existing order.
    @Transactional
    public Order checkout(String userId, String idempotencyKey) {
        // Idempotency guard — return existing order if key already used.
        Optional<Order> existing = orderRepo.findByIdempotencyKey(idempotencyKey);
        if (existing.isPresent()) {
            log.info("checkout.idempotent userId={} key={} orderId={}", userId, idempotencyKey, existing.get().getId());
            return existing.get();
        }

        Cart cart = cartService.getCart(userId);
        if (cart.getItems().isEmpty()) {
            throw new IllegalStateException("Cart is empty for userId=" + userId);
        }

        List<CartItem> items = cart.getItems();

        // Build order record first (status=PENDING) so we have an ID for saga.
        Order order = new Order();
        order.setUserId(userId);
        order.setStatus(OrderStatus.PENDING);
        order.setTotal(cart.getTotal());
        order.setIdempotencyKey(idempotencyKey);
        for (CartItem ci : items) {
            OrderItem oi = new OrderItem();
            oi.setOrder(order);
            oi.setProductId(ci.getProductId());
            oi.setSku(ci.getSku());
            oi.setName(ci.getName());
            oi.setQuantity(ci.getQuantity());
            oi.setUnitPrice(ci.getUnitPrice());
            order.getItems().add(oi);
        }
        order = orderRepo.save(order);
        log.info("checkout.order_created userId={} orderId={} total={}", userId, order.getId(), order.getTotal());

        // Reserve inventory atomically. Rolls back order on InsufficientStockException.
        try {
            inventoryService.reserve(order.getId(), items);
        } catch (InventoryService.InsufficientStockException e) {
            order.setStatus(OrderStatus.INVENTORY_FAILED);
            orderRepo.save(order);
            log.warn("checkout.inventory_failed orderId={} reason={}", order.getId(), e.getMessage());
            throw e;
        }

        order.setStatus(OrderStatus.INVENTORY_RESERVED);
        order = orderRepo.save(order);

        // Clear cart after successful reservation.
        cartService.clearCart(userId);

        // Publish event to trigger async payment step.
        eventPublisher.publishOrderCreated(order);
        log.info("checkout.published orderId={}", order.getId());

        return order;
    }
}
