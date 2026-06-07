package com.ankitsriv89.ecommerce.repository;

import com.ankitsriv89.ecommerce.domain.Product;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Modifying;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository
public interface ProductRepository extends JpaRepository<Product, String> {

    Optional<Product> findBySku(String sku);

    List<Product> findByCategory(String category);

    // Atomic decrement guarded by stock >= quantity to prevent oversell.
    @Modifying
    @Query("UPDATE Product p SET p.stock = p.stock - :qty WHERE p.id = :id AND p.stock >= :qty")
    int decrementStock(@Param("id") String id, @Param("qty") int qty);

    // Atomic increment for reservation release / admin restock.
    @Modifying
    @Query("UPDATE Product p SET p.stock = p.stock + :qty WHERE p.id = :id")
    int incrementStock(@Param("id") String id, @Param("qty") int qty);

    @Query("SELECT p FROM Product p WHERE p.stock > 0 ORDER BY p.createdAt DESC")
    List<Product> findInStock();
}
