package com.ankitsriv89.ecommerce.api;

import com.ankitsriv89.ecommerce.domain.Order;
import com.ankitsriv89.ecommerce.domain.OrderStatus;
import com.ankitsriv89.ecommerce.service.CatalogService;
import com.ankitsriv89.ecommerce.service.OrderService;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/v1/admin")
public class AdminController {

    private final OrderService orderService;
    private final CatalogService catalogService;

    public AdminController(OrderService orderService, CatalogService catalogService) {
        this.orderService = orderService;
        this.catalogService = catalogService;
    }

    @GetMapping("/orders")
    public List<Order> listAllOrders() {
        return orderService.listAll();
    }

    @PostMapping("/orders/{id}/ship")
    public Order shipOrder(@PathVariable String id) {
        return orderService.transition(id, OrderStatus.SHIPPED);
    }

    @PostMapping("/orders/{id}/deliver")
    public Order deliverOrder(@PathVariable String id) {
        return orderService.transition(id, OrderStatus.DELIVERED);
    }

    @PostMapping("/products/{id}/restock")
    public void restock(@PathVariable String id, @RequestParam int qty) {
        catalogService.adminRestock(id, qty);
    }
}
