package com.ankitsriv89.whatsapp.domain;

public enum ReceiptState {
    SENT,
    DELIVERED,
    READ;

    public boolean canAdvanceTo(ReceiptState next) {
        return next.ordinal() > this.ordinal();
    }
}
