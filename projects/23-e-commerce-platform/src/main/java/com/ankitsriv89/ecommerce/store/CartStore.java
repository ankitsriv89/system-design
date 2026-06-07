package com.ankitsriv89.ecommerce.store;

import com.ankitsriv89.ecommerce.domain.Cart;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Component;

import java.time.Duration;
import java.util.Optional;

@Component
public class CartStore {

    private static final Logger log = LoggerFactory.getLogger(CartStore.class);
    private static final Duration TTL = Duration.ofDays(7);
    private static final String PREFIX = "cart:";

    private final StringRedisTemplate redis;
    private final ObjectMapper mapper;

    public CartStore(StringRedisTemplate redis, ObjectMapper mapper) {
        this.redis = redis;
        this.mapper = mapper;
    }

    public Optional<Cart> get(String userId) {
        try {
            String json = redis.opsForValue().get(PREFIX + userId);
            if (json == null) return Optional.empty();
            return Optional.of(mapper.readValue(json, Cart.class));
        } catch (Exception e) {
            log.warn("cart.get.failed userId={}", userId, e);
            return Optional.empty();
        }
    }

    public void save(Cart cart) {
        try {
            String json = mapper.writeValueAsString(cart);
            redis.opsForValue().set(PREFIX + cart.getUserId(), json, TTL);
        } catch (Exception e) {
            log.error("cart.save.failed userId={}", cart.getUserId(), e);
        }
    }

    public void delete(String userId) {
        redis.delete(PREFIX + userId);
    }
}
