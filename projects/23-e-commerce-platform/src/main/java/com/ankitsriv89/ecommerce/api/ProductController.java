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
    public Product update(@PathVariable String id, @Valid @RequestBody Product product) {
        product.setId(id);
        return catalogService.updateProduct(product);
    }
}
