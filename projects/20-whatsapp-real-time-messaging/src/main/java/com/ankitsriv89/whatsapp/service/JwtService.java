package com.ankitsriv89.whatsapp.service;

import io.jsonwebtoken.*;
import io.jsonwebtoken.security.Keys;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import javax.crypto.SecretKey;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.Date;

@Service
public class JwtService {

    private static final Logger log = LoggerFactory.getLogger(JwtService.class);
    // HS256 requires ≥ 32 bytes; anything shorter is a misconfiguration.
    private static final int MIN_SECRET_BYTES = 32;
    private static final String DEV_SECRET_PREFIX = "whatsapp-dev-secret";

    private final SecretKey key;
    private final long expiryMinutes;

    public JwtService(
            @Value("${whatsapp.jwt.secret}") String secret,
            @Value("${whatsapp.jwt.expiry-minutes:1440}") long expiryMinutes) {
        byte[] bytes = secret.getBytes(StandardCharsets.UTF_8);
        if (bytes.length < MIN_SECRET_BYTES) {
            throw new IllegalStateException(
                    "JWT secret is too short (" + bytes.length + " bytes); must be ≥ " + MIN_SECRET_BYTES +
                    " bytes. Set JWT_SECRET env var.");
        }
        if (secret.startsWith(DEV_SECRET_PREFIX)) {
            log.warn("JWT secret is the default development placeholder — set JWT_SECRET in production");
        }
        this.key = Keys.hmacShaKeyFor(bytes);
        this.expiryMinutes = expiryMinutes;
    }

    public String generate(String username, Long userId) {
        Instant now = Instant.now();
        return Jwts.builder()
                .subject(username)
                .claim("uid", userId)
                .issuedAt(Date.from(now))
                .expiration(Date.from(now.plusSeconds(expiryMinutes * 60)))
                .signWith(key)
                .compact();
    }

    public Claims parse(String token) {
        return Jwts.parser().verifyWith(key).build()
                .parseSignedClaims(token).getPayload();
    }

    public boolean isValid(String token) {
        try {
            parse(token);
            return true;
        } catch (JwtException | IllegalArgumentException e) {
            return false;
        }
    }

    public String extractUsername(String token) { return parse(token).getSubject(); }
    public Long extractUserId(String token) { return parse(token).get("uid", Long.class); }
}
