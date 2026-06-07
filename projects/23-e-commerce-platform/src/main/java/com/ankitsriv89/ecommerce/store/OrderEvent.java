package com.ankitsriv89.ecommerce.store;

import com.ankitsriv89.ecommerce.domain.OrderStatus;
import java.math.BigDecimal;
import java.time.Instant;

public class OrderEvent {
    private String orderId;
    private String userId;
    private OrderStatus status;
    private BigDecimal total;
    private Instant occurredAt;

    public OrderEvent() {}

    public OrderEvent(String orderId, String userId, OrderStatus status, BigDecimal total) {
        this.orderId = orderId;
        this.userId = userId;
        this.status = status;
        this.total = total;
        this.occurredAt = Instant.now();
    }

    public String getOrderId() { return orderId; }
    public void setOrderId(String orderId) { this.orderId = orderId; }
    public String getUserId() { return userId; }
    public void setUserId(String userId) { this.userId = userId; }
    public OrderStatus getStatus() { return status; }
    public void setStatus(OrderStatus status) { this.status = status; }
    public BigDecimal getTotal() { return total; }
    public void setTotal(BigDecimal total) { this.total = total; }
    public Instant getOccurredAt() { return occurredAt; }
    public void setOccurredAt(Instant occurredAt) { this.occurredAt = occurredAt; }
}
