package com.ankitsriv89.ecommerce.service;

import com.ankitsriv89.ecommerce.domain.Cart;
import com.ankitsriv89.ecommerce.domain.CartItem;
import com.ankitsriv89.ecommerce.domain.Product;
import com.ankitsriv89.ecommerce.repository.ProductRepository;
import com.ankitsriv89.ecommerce.store.CartStore;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;

@Service
public class CartService {

    private static final Logger log = LoggerFactory.getLogger(CartService.class);

    private final CartStore cartStore;
    private final ProductRepository productRepo;

    public CartService(CartStore cartStore, ProductRepository productRepo) {
        this.cartStore = cartStore;
        this.productRepo = productRepo;
    }

    public Cart getCart(String userId) {
        return cartStore.get(userId).orElseGet(() -> new Cart(userId));
    }

    public Cart addItem(String userId, String productId, int quantity) {
        Product p = productRepo.findById(productId)
                .orElseThrow(() -> new IllegalArgumentException("Product not found: " + productId));

        Cart cart = getCart(userId);
        cart.addOrUpdate(new CartItem(p.getId(), p.getSku(), p.getName(), quantity, p.getPrice()));
        cartStore.save(cart);
        log.info("cart.add userId={} productId={} qty={}", userId, productId, quantity);
        return cart;
    }

    public Cart removeItem(String userId, String productId) {
        Cart cart = getCart(userId);
        cart.remove(productId);
        cartStore.save(cart);
        log.info("cart.remove userId={} productId={}", userId, productId);
        return cart;
    }

    public void clearCart(String userId) {
        cartStore.delete(userId);
        log.info("cart.clear userId={}", userId);
    }
}
