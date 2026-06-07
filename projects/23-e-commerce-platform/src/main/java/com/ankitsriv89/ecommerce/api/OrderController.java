package com.ankitsriv89.ecommerce.api;

import com.ankitsriv89.ecommerce.domain.Order;
import com.ankitsriv89.ecommerce.service.CheckoutService;
import com.ankitsriv89.ecommerce.service.InventoryService;
import com.ankitsriv89.ecommerce.service.OrderService;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.List;
import java.util.UUID;

@RestController
@RequestMapping("/v1/orders")
public class OrderController {

    private final CheckoutService checkoutService;
    private final OrderService orderService;

    public OrderController(CheckoutService checkoutService, OrderService orderService) {
        this.checkoutService = checkoutService;
        this.orderService = orderService;
    }

    @PostMapping
    @ResponseStatus(HttpStatus.CREATED)
    public ResponseEntity<?> checkout(@RequestBody CheckoutRequest req) {
        try {
            String key = req.idempotencyKey() != null ? req.idempotencyKey() : UUID.randomUUID().toString();
            Order order = checkoutService.checkout(req.userId(), key);
            return ResponseEntity.status(HttpStatus.CREATED).body(order);
        } catch (InventoryService.InsufficientStockException e) {
            return ResponseEntity.status(HttpStatus.CONFLICT).body(new ErrorResponse(e.getMessage()));
        } catch (IllegalStateException e) {
            return ResponseEntity.badRequest().body(new ErrorResponse(e.getMessage()));
        }
    }

    @GetMapping("/{id}")
    public ResponseEntity<Order> getOrder(@PathVariable String id) {
        return orderService.getOrder(id)
                .map(ResponseEntity::ok)
                .orElse(ResponseEntity.notFound().build());
    }

    @GetMapping
    public List<Order> listOrders(@RequestParam String userId) {
        return orderService.getOrdersByUser(userId);
    }

    record CheckoutRequest(String userId, String idempotencyKey) {}
    record ErrorResponse(String error) {}
}
