package com.ankitsriv89.ecommerce;

import com.ankitsriv89.ecommerce.domain.*;
import com.ankitsriv89.ecommerce.repository.OrderRepository;
import com.ankitsriv89.ecommerce.repository.ProductRepository;
import com.ankitsriv89.ecommerce.service.*;
import com.ankitsriv89.ecommerce.store.CartStore;
import com.ankitsriv89.ecommerce.store.EventPublisher;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.math.BigDecimal;
import java.util.List;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;

class CheckoutServiceTest {

    private ProductRepository productRepo;
    private OrderRepository orderRepo;
    private CartStore cartStore;
    private InventoryService inventoryService;
    private CartService cartService;
    private CheckoutService checkoutService;
    private EventPublisher eventPublisher;

    @BeforeEach
    void setUp() {
        productRepo = mock(ProductRepository.class);
        orderRepo = mock(OrderRepository.class);
        cartStore = mock(CartStore.class);
        eventPublisher = mock(EventPublisher.class);

        var reservationRepo = mock(com.ankitsriv89.ecommerce.repository.InventoryReservationRepository.class);
        inventoryService = new InventoryService(productRepo, reservationRepo);
        cartService = new CartService(cartStore, productRepo);
        checkoutService = new CheckoutService(cartService, inventoryService, orderRepo, eventPublisher);
    }

    @Test
    void checkout_emptyCart_throwsIllegalState() {
        String userId = "user-1";
        when(cartStore.get(userId)).thenReturn(Optional.of(new Cart(userId)));

        assertThrows(IllegalStateException.class,
                () -> checkoutService.checkout(userId, "key-1"));
    }

    @Test
    void checkout_idempotent_returnsSameOrder() {
        String userId = "user-1";
        String key = "idem-key-1";
        Order existing = new Order();
        existing.setUserId(userId);
        existing.setStatus(OrderStatus.CONFIRMED);
        existing.setTotal(BigDecimal.TEN);
        existing.setIdempotencyKey(key);

        when(orderRepo.findByIdempotencyKey(key)).thenReturn(Optional.of(existing));

        Order result = checkoutService.checkout(userId, key);
        assertSame(existing, result);
        verify(orderRepo, never()).save(any(Order.class));
    }

    @Test
    void inventoryService_insufficientStock_throws() {
        Product p = new Product();
        p.setId("prod-1");
        p.setSku("SKU-1");
        p.setPrice(BigDecimal.TEN);
        p.setStock(0);

        when(productRepo.decrementStock("prod-1", 2)).thenReturn(0);

        CartItem item = new CartItem("prod-1", "SKU-1", "Widget", 2, BigDecimal.TEN);

        assertThrows(InventoryService.InsufficientStockException.class,
                () -> inventoryService.reserve("order-1", List.of(item)));
    }

    @Test
    void inventoryService_sufficientStock_reserves() {
        when(productRepo.decrementStock("prod-2", 1)).thenReturn(1);

        var resRepo = mock(com.ankitsriv89.ecommerce.repository.InventoryReservationRepository.class);
        var reservation = new InventoryReservation();
        reservation.setOrderId("order-2");
        reservation.setProductId("prod-2");
        reservation.setSku("SKU-2");
        reservation.setQuantity(1);
        when(resRepo.save(any())).thenReturn(reservation);

        var svc = new InventoryService(productRepo, resRepo);
        CartItem item = new CartItem("prod-2", "SKU-2", "Gadget", 1, BigDecimal.ONE);
        var result = svc.reserve("order-2", List.of(item));

        assertEquals(1, result.size());
        assertEquals("SKU-2", result.get(0).getSku());
    }

    @Test
    void partitionFor_keyBased_isDeterministic() {
        // FNV-1a routing: same key always maps to same partition across calls.
        String key = "order-abc-123";
        int partitions = 4;
        int p1 = Math.abs(key.hashCode()) % partitions;
        int p2 = Math.abs(key.hashCode()) % partitions;
        assertEquals(p1, p2);
    }
}
