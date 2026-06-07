package com.ankitsriv89.ecommerce.api;

import com.ankitsriv89.ecommerce.domain.Product;
import com.ankitsriv89.ecommerce.service.CatalogService;
import jakarta.validation.Valid;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/v1/products")
public class ProductController {

    private final CatalogService catalogService;

    public ProductController(CatalogService catalogService) {
        this.catalogService = catalogService;
    }

    @GetMapping
    public List<Product> list(@RequestParam(required = false) String category,
                               @RequestParam(required = false) String q) {
        if (q != null && !q.isBlank()) return catalogService.search(q);
        if (category != null && !category.isBlank()) return catalogService.listByCategory(category);
        return catalogService.listProducts();
    }

    @GetMapping("/{id}")
    public ResponseEntity<Product> get(@PathVariable String id) {
        return catalogService.getProduct(id)
                .map(ResponseEntity::ok)
                .orElse(ResponseEntity.notFound().build());
    }

    @PostMapping
    @ResponseStatus(HttpStatus.CREATED)
    public Product create(@Valid @RequestBody Product product) {
        return catalogService.createProduct(product);
    }

    @PutMapping("/{id}")
    public ResponseEntity<Product> update(@PathVariable String id,
                                          @Valid @RequestBody ProductUpdateRequest req) {
        return catalogService.getProduct(id).map(existing -> {
            // Patch only mutable fields — never accept id/createdAt from request body.
            if (req.name() != null)        existing.setName(req.name());
            if (req.description() != null) existing.setDescription(req.description());
            if (req.price() != null)       existing.setPrice(req.price());
            if (req.category() != null)    existing.setCategory(req.category());
            if (req.imageUrl() != null)    existing.setImageUrl(req.imageUrl());
            return ResponseEntity.ok(catalogService.updateProduct(existing));
        }).orElse(ResponseEntity.notFound().build());
    }

    record ProductUpdateRequest(
        String name,
        String description,
        java.math.BigDecimal price,
        String category,
        String imageUrl
    ) {}
}
