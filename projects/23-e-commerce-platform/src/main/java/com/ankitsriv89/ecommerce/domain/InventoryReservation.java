package com.ankitsriv89.ecommerce.domain;

import jakarta.persistence.*;
import java.time.Instant;

@Entity
@Table(name = "inventory_reservations",
       indexes = @Index(name = "idx_reservation_order", columnList = "order_id"))
public class InventoryReservation {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "order_id", nullable = false)
    private String orderId;

    @Column(name = "product_id", nullable = false)
    private String productId;

    @Column(name = "sku", nullable = false)
    private String sku;

    @Column(name = "quantity", nullable = false)
    private int quantity;

    @Column(name = "reserved_at", nullable = false)
    private Instant reservedAt;

    @Column(name = "released_at")
    private Instant releasedAt;

    @PrePersist
    void onCreate() { reservedAt = Instant.now(); }

    public Long getId() { return id; }
    public String getOrderId() { return orderId; }
    public void setOrderId(String orderId) { this.orderId = orderId; }
    public String getProductId() { return productId; }
    public void setProductId(String productId) { this.productId = productId; }
    public String getSku() { return sku; }
    public void setSku(String sku) { this.sku = sku; }
    public int getQuantity() { return quantity; }
    public void setQuantity(int quantity) { this.quantity = quantity; }
    public Instant getReservedAt() { return reservedAt; }
    public Instant getReleasedAt() { return releasedAt; }
    public void setReleasedAt(Instant releasedAt) { this.releasedAt = releasedAt; }
}
