package com.ankitsriv89.whatsapp.config;

import com.ankitsriv89.whatsapp.service.JwtService;
import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.http.HttpStatus;
import org.springframework.security.authentication.UsernamePasswordAuthenticationToken;
import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.config.annotation.web.configuration.EnableWebSecurity;
import org.springframework.security.config.annotation.web.configurers.AbstractHttpConfigurer;
import org.springframework.security.config.http.SessionCreationPolicy;
import org.springframework.security.core.authority.SimpleGrantedAuthority;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.security.web.SecurityFilterChain;
import org.springframework.security.web.authentication.UsernamePasswordAuthenticationFilter;
import org.springframework.web.filter.OncePerRequestFilter;

import java.io.IOException;
import java.util.List;

@Configuration
@EnableWebSecurity
public class SecurityConfig {

    private final JwtService jwt;

    public SecurityConfig(JwtService jwt) { this.jwt = jwt; }

    @Bean
    public SecurityFilterChain chain(HttpSecurity http) throws Exception {
        http
            .csrf(AbstractHttpConfigurer::disable)
            .sessionManagement(s -> s.sessionCreationPolicy(SessionCreationPolicy.STATELESS))
            .authorizeHttpRequests(a -> a
                .requestMatchers(
                    "/v1/auth/**",
                    "/ws/**",
                    "/actuator/health",
                    "/actuator/prometheus",
                    "/",
                    "/index.html",
                    "/styles.css",
                    "/app.js",
                    "/web/**"
                ).permitAll()
                // ws-ticket requires a valid JWT (authenticated) — explicitly listed for clarity
                .requestMatchers("/v1/ws-ticket").authenticated()
                .anyRequest().authenticated()
            )
            .addFilterBefore(jwtFilter(), UsernamePasswordAuthenticationFilter.class);
        return http.build();
    }

    @Bean
    public PasswordEncoder passwordEncoder() { return new BCryptPasswordEncoder(); }

    private OncePerRequestFilter jwtFilter() {
        return new OncePerRequestFilter() {
            @Override
            protected void doFilterInternal(HttpServletRequest req, HttpServletResponse res, FilterChain chain)
                    throws ServletException, IOException {
                String header = req.getHeader("Authorization");
                if (header != null && header.startsWith("Bearer ")) {
                    String token = header.substring(7);
                    if (jwt.isValid(token)) {
                        String username = jwt.extractUsername(token);
                        Long userId = jwt.extractUserId(token);
                        var auth = new UsernamePasswordAuthenticationToken(
                                username, null,
                                List.of(new SimpleGrantedAuthority("ROLE_USER")));
                        auth.setDetails(userId);
                        SecurityContextHolder.getContext().setAuthentication(auth);
                    }
                }
                chain.doFilter(req, res);
            }
        };
    }
}
