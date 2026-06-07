package com.ankitsriv89.ecommerce.domain;

import java.math.BigDecimal;

public class CartItem {
    private String productId;
    private String sku;
    private String name;
    private int quantity;
    private BigDecimal unitPrice;

    public CartItem() {}

    public CartItem(String productId, String sku, String name, int quantity, BigDecimal unitPrice) {
        this.productId = productId;
        this.sku = sku;
        this.name = name;
        this.quantity = quantity;
        this.unitPrice = unitPrice;
    }

    public String getProductId() { return productId; }
    public void setProductId(String productId) { this.productId = productId; }
    public String getSku() { return sku; }
    public void setSku(String sku) { this.sku = sku; }
    public String getName() { return name; }
    public void setName(String name) { this.name = name; }
    public int getQuantity() { return quantity; }
    public void setQuantity(int quantity) { this.quantity = quantity; }
    public BigDecimal getUnitPrice() { return unitPrice; }
    public void setUnitPrice(BigDecimal unitPrice) { this.unitPrice = unitPrice; }
    public BigDecimal getLineTotal() { return unitPrice.multiply(BigDecimal.valueOf(quantity)); }
}
