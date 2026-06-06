package com.ankitsriv89.whatsapp.service;

import com.ankitsriv89.whatsapp.domain.AppUser;
import com.ankitsriv89.whatsapp.dto.AuthResponse;
import com.ankitsriv89.whatsapp.dto.LoginRequest;
import com.ankitsriv89.whatsapp.dto.RegisterRequest;
import com.ankitsriv89.whatsapp.repository.UserRepository;
import org.springframework.security.authentication.BadCredentialsException;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class AuthService {

    private final UserRepository users;
    private final PasswordEncoder encoder;
    private final JwtService jwt;
    // password hash stored separately since AppUser is server-side only
    private final java.util.concurrent.ConcurrentHashMap<String, String> passwordMap = new java.util.concurrent.ConcurrentHashMap<>();

    public AuthService(UserRepository users, PasswordEncoder encoder, JwtService jwt) {
        this.users = users;
        this.encoder = encoder;
        this.jwt = jwt;
    }

    @Transactional
    public AuthResponse register(RegisterRequest req) {
        if (users.existsByUsername(req.username())) {
            throw new IllegalArgumentException("Username already taken");
        }
        AppUser user = users.save(new AppUser(req.username()));
        passwordMap.put(req.username(), encoder.encode(req.password()));
        return new AuthResponse(jwt.generate(user.getUsername(), user.getId()), user.getId(), user.getUsername());
    }

    public AuthResponse login(LoginRequest req) {
        AppUser user = users.findByUsername(req.username())
                .orElseThrow(() -> new BadCredentialsException("Invalid credentials"));
        String hash = passwordMap.get(req.username());
        if (hash == null || !encoder.matches(req.password(), hash)) {
            throw new BadCredentialsException("Invalid credentials");
        }
        return new AuthResponse(jwt.generate(user.getUsername(), user.getId()), user.getId(), user.getUsername());
    }
}
