package com.ankitsriv89.groupchat.controller;

import com.ankitsriv89.groupchat.config.JwtUtil;
import com.ankitsriv89.groupchat.dto.TokenResponse;
import org.springframework.web.bind.annotation.*;

import java.util.Map;

@RestController
@RequestMapping("/api/auth")
public class AuthController {

    private final JwtUtil jwt;

    public AuthController(JwtUtil jwt) {
        this.jwt = jwt;
    }

        // Demo-mode token minting: accepts a userId without password verification.
    // This is intentional for the tutorial UI — there is no user store in this project.
    // In production: verify against a password hash (bcrypt/argon2) or delegate to an OIDC provider.
    @PostMapping("/token")
    public TokenResponse token(@RequestBody Map<String, String> body) {
        String userId = body.get("userId");
        if (userId == null || userId.isBlank()) throw new IllegalArgumentException("userId required");
        return new TokenResponse(jwt.generate(userId.trim()), userId.trim());
    }
}
