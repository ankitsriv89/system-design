package com.ankitsriv89.ecommerce.service;

import com.ankitsriv89.ecommerce.domain.Product;
import com.ankitsriv89.ecommerce.repository.ProductRepository;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.cache.annotation.CacheEvict;
import org.springframework.cache.annotation.Cacheable;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;
import java.util.Optional;

@Service
public class CatalogService {

    private static final Logger log = LoggerFactory.getLogger(CatalogService.class);

    private final ProductRepository productRepo;

    public CatalogService(ProductRepository productRepo) {
        this.productRepo = productRepo;
    }

    @Transactional
    public Product createProduct(Product p) {
        log.info("catalog.create sku={}", p.getSku());
        return productRepo.save(p);
    }

    @Cacheable(value = "product", key = "#id")
    @Transactional(readOnly = true)
    public Optional<Product> getProduct(String id) {
        return productRepo.findById(id);
    }

    @Transactional(readOnly = true)
    public List<Product> listProducts() {
        return productRepo.findAll();
    }

    @Transactional(readOnly = true)
    public List<Product> listByCategory(String category) {
        return productRepo.findByCategory(category);
    }

    @Transactional(readOnly = true)
    public List<Product> search(String query) {
        // Fallback: simple DB LIKE search when OpenSearch is unavailable.
        return productRepo.findAll().stream()
                .filter(p -> p.getName().toLowerCase().contains(query.toLowerCase())
                          || (p.getDescription() != null && p.getDescription().toLowerCase().contains(query.toLowerCase())))
                .toList();
    }

    @CacheEvict(value = "product", key = "#p.id")
    @Transactional
    public Product updateProduct(Product p) {
        log.info("catalog.update id={}", p.getId());
        return productRepo.save(p);
    }

    @Transactional
    public void adminRestock(String productId, int qty) {
        log.info("catalog.restock id={} qty={}", productId, qty);
        productRepo.incrementStock(productId, qty);
    }
}
