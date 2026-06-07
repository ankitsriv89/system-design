package com.ankitsriv89.ecommerce.repository;

import com.ankitsriv89.ecommerce.domain.InventoryReservation;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
public interface InventoryReservationRepository extends JpaRepository<InventoryReservation, Long> {

    List<InventoryReservation> findByOrderId(String orderId);

    List<InventoryReservation> findByOrderIdAndReleasedAtIsNull(String orderId);
}
