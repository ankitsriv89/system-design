package com.ankitsriv89.ecommerce.domain;

import java.math.BigDecimal;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;

// Stored as JSON in Redis; not a JPA entity.
public class Cart {
    private String userId;
    private List<CartItem> items = new ArrayList<>();
    private Instant updatedAt;

    public Cart() {}

    public Cart(String userId) {
        this.userId = userId;
        this.updatedAt = Instant.now();
    }

    public void addOrUpdate(CartItem incoming) {
        items.stream()
             .filter(i -> i.getProductId().equals(incoming.getProductId()))
             .findFirst()
             .ifPresentOrElse(
                 i -> i.setQuantity(i.getQuantity() + incoming.getQuantity()),
                 () -> items.add(incoming));
        updatedAt = Instant.now();
    }

    public void remove(String productId) {
        items.removeIf(i -> i.getProductId().equals(productId));
        updatedAt = Instant.now();
    }

    public BigDecimal getTotal() {
        return items.stream()
                    .map(CartItem::getLineTotal)
                    .reduce(BigDecimal.ZERO, BigDecimal::add);
    }

    public String getUserId() { return userId; }
    public void setUserId(String userId) { this.userId = userId; }
    public List<CartItem> getItems() { return items; }
    public void setItems(List<CartItem> items) { this.items = items; }
    public Instant getUpdatedAt() { return updatedAt; }
    public void setUpdatedAt(Instant updatedAt) { this.updatedAt = updatedAt; }
}
