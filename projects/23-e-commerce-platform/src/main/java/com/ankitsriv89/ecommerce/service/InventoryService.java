package com.ankitsriv89.ecommerce.service;

import com.ankitsriv89.ecommerce.domain.CartItem;
import com.ankitsriv89.ecommerce.domain.InventoryReservation;
import com.ankitsriv89.ecommerce.repository.InventoryReservationRepository;
import com.ankitsriv89.ecommerce.repository.ProductRepository;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.Instant;
import java.util.ArrayList;
import java.util.List;

@Service
public class InventoryService {

    private static final Logger log = LoggerFactory.getLogger(InventoryService.class);

    private final ProductRepository productRepo;
    private final InventoryReservationRepository reservationRepo;

    public InventoryService(ProductRepository productRepo,
                             InventoryReservationRepository reservationRepo) {
        this.productRepo = productRepo;
        this.reservationRepo = reservationRepo;
    }

    // Atomically reserves inventory for all items in one transaction.
    // Rolls back entirely if any item is out of stock (no partial reservations).
    @Transactional
    public List<InventoryReservation> reserve(String orderId, List<CartItem> items) {
        List<InventoryReservation> reservations = new ArrayList<>();
        for (CartItem item : items) {
            int updated = productRepo.decrementStock(item.getProductId(), item.getQuantity());
            if (updated == 0) {
                // decrementStock returns 0 when stock < qty — triggers rollback via exception.
                throw new InsufficientStockException(
                        "Insufficient stock for SKU " + item.getSku()
                        + " (requested " + item.getQuantity() + ")");
            }
            InventoryReservation res = new InventoryReservation();
            res.setOrderId(orderId);
            res.setProductId(item.getProductId());
            res.setSku(item.getSku());
            res.setQuantity(item.getQuantity());
            reservations.add(reservationRepo.save(res));
            log.info("inventory.reserved orderId={} sku={} qty={}", orderId, item.getSku(), item.getQuantity());
        }
        return reservations;
    }

    // Releases all active reservations for an order (compensation step).
    @Transactional
    public void release(String orderId) {
        List<InventoryReservation> active = reservationRepo.findByOrderIdAndReleasedAtIsNull(orderId);
        for (InventoryReservation res : active) {
            productRepo.incrementStock(res.getProductId(), res.getQuantity());
            res.setReleasedAt(Instant.now());
            reservationRepo.save(res);
            log.info("inventory.released orderId={} sku={} qty={}", orderId, res.getSku(), res.getQuantity());
        }
    }

    public static class InsufficientStockException extends RuntimeException {
        public InsufficientStockException(String msg) { super(msg); }
    }
}
