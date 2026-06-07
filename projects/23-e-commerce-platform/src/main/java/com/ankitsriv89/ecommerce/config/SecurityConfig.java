package com.ankitsriv89.ecommerce.config;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.http.HttpMethod;
import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.config.annotation.web.configuration.EnableWebSecurity;
import org.springframework.security.config.annotation.web.configurers.AbstractHttpConfigurer;
import org.springframework.security.core.userdetails.User;
import org.springframework.security.core.userdetails.UserDetailsService;
import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.security.provisioning.InMemoryUserDetailsManager;
import org.springframework.security.web.SecurityFilterChain;

@Configuration
@EnableWebSecurity
public class SecurityConfig {

    // Admin credentials — override via ADMIN_USER / ADMIN_PASSWORD env vars in production.
    // HTTP Basic is used here so the demo UI and scripts work without a login flow.
    // In production: replace with JWT/OAuth2 and derive userId from the access token principal.

    @Bean
    public PasswordEncoder passwordEncoder() {
        return new BCryptPasswordEncoder();
    }

    @Bean
    public UserDetailsService userDetailsService(PasswordEncoder encoder) {
        String adminUser = System.getenv().getOrDefault("ADMIN_USER", "admin");
        String adminPass = System.getenv().getOrDefault("ADMIN_PASSWORD", "admin");
        return new InMemoryUserDetailsManager(
            User.builder()
                .username(adminUser)
                .password(encoder.encode(adminPass))
                .roles("ADMIN")
                .build()
        );
    }

    @Bean
    public SecurityFilterChain filterChain(HttpSecurity http) throws Exception {
        http.csrf(AbstractHttpConfigurer::disable)
            .authorizeHttpRequests(auth -> auth
                // Admin operations: ship, deliver, restock, list all orders — require ADMIN role.
                .requestMatchers("/v1/admin/**").hasRole("ADMIN")
                // Product writes — require ADMIN role (reads remain open for the catalog UI).
                .requestMatchers(HttpMethod.POST, "/v1/products").hasRole("ADMIN")
                .requestMatchers(HttpMethod.PUT,  "/v1/products/**").hasRole("ADMIN")
                // Everything else open (catalog reads, cart, checkout, health, metrics, UI).
                .anyRequest().permitAll()
            )
            .httpBasic(basic -> {});
        return http.build();
    }
}
