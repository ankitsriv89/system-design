package com.ankitsriv89.ecommerce.api;

import com.ankitsriv89.ecommerce.domain.Cart;
import com.ankitsriv89.ecommerce.service.CartService;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/v1/cart")
public class CartController {

    private final CartService cartService;

    public CartController(CartService cartService) {
        this.cartService = cartService;
    }

    @GetMapping("/{userId}")
    public Cart getCart(@PathVariable String userId) {
        return cartService.getCart(userId);
    }

    @PostMapping("/{userId}/items")
    public Cart addItem(@PathVariable String userId,
                        @RequestBody AddItemRequest req) {
        return cartService.addItem(userId, req.productId(), req.quantity());
    }

    @DeleteMapping("/{userId}/items/{productId}")
    public Cart removeItem(@PathVariable String userId,
                           @PathVariable String productId) {
        return cartService.removeItem(userId, productId);
    }

    @DeleteMapping("/{userId}")
    public ResponseEntity<Void> clearCart(@PathVariable String userId) {
        cartService.clearCart(userId);
        return ResponseEntity.noContent().build();
    }

    record AddItemRequest(String productId, int quantity) {}
}
