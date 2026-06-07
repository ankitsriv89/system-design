package com.ankitsriv89.ecommerce.domain;

public enum OrderStatus {
    PENDING,
    INVENTORY_RESERVED,
    PAYMENT_AUTHORIZED,
    CONFIRMED,
    SHIPPED,
    DELIVERED,
    CANCELLED,
    PAYMENT_FAILED,
    INVENTORY_FAILED
}
