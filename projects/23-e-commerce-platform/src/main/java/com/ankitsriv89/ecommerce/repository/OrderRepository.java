package com.ankitsriv89.ecommerce.repository;

import com.ankitsriv89.ecommerce.domain.Order;
import com.ankitsriv89.ecommerce.domain.OrderStatus;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository
public interface OrderRepository extends JpaRepository<Order, String> {

    List<Order> findByUserId(String userId);

    List<Order> findByStatus(OrderStatus status);

    Optional<Order> findByIdempotencyKey(String idempotencyKey);
}
