package com.ankitsriv89.ecommerce.api;

import com.ankitsriv89.ecommerce.domain.Cart;
import com.ankitsriv89.ecommerce.service.CartService;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

// IDOR guard: callers must supply X-User-Id matching the path userId.
// In production, replace with JWT principal — derive userId from the verified token
// and remove the path param entirely (userId = Authentication.getName()).
@RestController
@RequestMapping("/v1/cart")
public class CartController {

    private final CartService cartService;

    public CartController(CartService cartService) {
        this.cartService = cartService;
    }

    @GetMapping("/{userId}")
    public ResponseEntity<Cart> getCart(@PathVariable String userId,
                                        @RequestHeader(value = "X-User-Id", required = false) String callerId) {
        if (!isSelf(callerId, userId)) return ResponseEntity.status(403).build();
        return ResponseEntity.ok(cartService.getCart(userId));
    }

    @PostMapping("/{userId}/items")
    public ResponseEntity<Cart> addItem(@PathVariable String userId,
                                        @RequestHeader(value = "X-User-Id", required = false) String callerId,
                                        @RequestBody AddItemRequest req) {
        if (!isSelf(callerId, userId)) return ResponseEntity.status(403).build();
        return ResponseEntity.ok(cartService.addItem(userId, req.productId(), req.quantity()));
    }

    @DeleteMapping("/{userId}/items/{productId}")
    public ResponseEntity<Cart> removeItem(@PathVariable String userId,
                                           @PathVariable String productId,
                                           @RequestHeader(value = "X-User-Id", required = false) String callerId) {
        if (!isSelf(callerId, userId)) return ResponseEntity.status(403).build();
        return ResponseEntity.ok(cartService.removeItem(userId, productId));
    }

    @DeleteMapping("/{userId}")
    public ResponseEntity<Void> clearCart(@PathVariable String userId,
                                          @RequestHeader(value = "X-User-Id", required = false) String callerId) {
        if (!isSelf(callerId, userId)) return ResponseEntity.status(403).build();
        cartService.clearCart(userId);
        return ResponseEntity.noContent().build();
    }

    private boolean isSelf(String callerId, String pathUserId) {
        return callerId != null && callerId.equals(pathUserId);
    }

    record AddItemRequest(String productId, int quantity) {}
}
