package com.ankitsriv89.chat.controller;

import com.ankitsriv89.chat.config.JwtUtil;
import com.ankitsriv89.chat.dto.TokenResponse;
import org.springframework.web.bind.annotation.*;

// Demo auth: issue a JWT for any requested userId with no password check.
// In production this would validate credentials against a user store.
@RestController
@RequestMapping("/api/v1/auth")
public class AuthController {

    private final JwtUtil jwt;

    public AuthController(JwtUtil jwt) {
        this.jwt = jwt;
    }

    @PostMapping("/token")
    public TokenResponse token(@RequestParam String userId) {
        if (userId == null || userId.isBlank() || userId.length() > 64) {
            throw new IllegalArgumentException("userId must be 1-64 chars");
        }
        return new TokenResponse(jwt.generate(userId.trim()), userId.trim());
    }
}
