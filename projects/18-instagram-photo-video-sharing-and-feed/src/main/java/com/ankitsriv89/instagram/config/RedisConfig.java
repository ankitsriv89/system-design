package com.ankitsriv89.instagram.config;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.data.redis.connection.RedisConnectionFactory;
import org.springframework.data.redis.core.StringRedisTemplate;

@Configuration
public class RedisConfig {

    // Home timelines are Redis sorted sets (ZSET): member = postId, score =
    // ranking score. Like counters are simple Redis strings. A StringRedisTemplate
    // covers ZADD/ZREVRANGE and INCR/DECR.
    @Bean
    public StringRedisTemplate stringRedisTemplate(RedisConnectionFactory cf) {
        return new StringRedisTemplate(cf);
    }
}
