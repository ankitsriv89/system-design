package com.ankitsriv89.whatsapp.api;

import com.ankitsriv89.whatsapp.dto.AuthResponse;
import com.ankitsriv89.whatsapp.dto.LoginRequest;
import com.ankitsriv89.whatsapp.dto.RegisterRequest;
import com.ankitsriv89.whatsapp.service.AuthService;
import jakarta.validation.Valid;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/v1/auth")
public class AuthController {

    private final AuthService auth;

    public AuthController(AuthService auth) { this.auth = auth; }

    @PostMapping("/register")
    public ResponseEntity<AuthResponse> register(@Valid @RequestBody RegisterRequest req) {
        return ResponseEntity.ok(auth.register(req));
    }

    @PostMapping("/login")
    public ResponseEntity<AuthResponse> login(@Valid @RequestBody LoginRequest req) {
        return ResponseEntity.ok(auth.login(req));
    }
}
