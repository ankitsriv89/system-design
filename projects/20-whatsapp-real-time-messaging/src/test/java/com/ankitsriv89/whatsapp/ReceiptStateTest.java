package com.ankitsriv89.whatsapp;

import com.ankitsriv89.whatsapp.domain.ReceiptState;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class ReceiptStateTest {

    @Test
    void sentCanAdvanceToDelivered() {
        assertTrue(ReceiptState.SENT.canAdvanceTo(ReceiptState.DELIVERED));
    }

    @Test
    void sentCanAdvanceToRead() {
        assertTrue(ReceiptState.SENT.canAdvanceTo(ReceiptState.READ));
    }

    @Test
    void deliveredCanAdvanceToRead() {
        assertTrue(ReceiptState.DELIVERED.canAdvanceTo(ReceiptState.READ));
    }

    @Test
    void readCannotAdvanceToDelivered() {
        assertFalse(ReceiptState.READ.canAdvanceTo(ReceiptState.DELIVERED));
    }

    @Test
    void deliveredCannotGoBackToSent() {
        assertFalse(ReceiptState.DELIVERED.canAdvanceTo(ReceiptState.SENT));
    }

    @Test
    void stateIsIdempotentForSameLevel() {
        assertFalse(ReceiptState.SENT.canAdvanceTo(ReceiptState.SENT));
    }
}
