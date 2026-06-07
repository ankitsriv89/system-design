package com.ankitsriv89.ticketbooking.config;

import com.ankitsriv89.ticketbooking.api.CallerIdFilter;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.config.annotation.web.configuration.EnableWebSecurity;
import org.springframework.security.config.annotation.web.configurers.AbstractHttpConfigurer;
import org.springframework.security.web.SecurityFilterChain;
import org.springframework.security.web.authentication.UsernamePasswordAuthenticationFilter;

// Trust model: this service is designed to run behind an auth gateway (Kong, Nginx, Spring Cloud
// Gateway) that validates JWT tokens and injects X-User-Id. The CallerIdFilter below enforces
// that X-User-Id is present and non-blank on /v1/** routes, but it does NOT cryptographically
// verify the header — it is trivially spoofable if the service is reachable directly.
//
// TODO (Milestone 3): replace CallerIdFilter with spring-boot-starter-oauth2-resource-server.
// Configure jwt.issuer-uri and derive the principal from JwtAuthenticationToken. Remove the
// X-User-Id header entirely once Spring Security provides the principal.
@Configuration
@EnableWebSecurity
public class SecurityConfig {

    @Bean
    public SecurityFilterChain filterChain(HttpSecurity http, CallerIdFilter callerIdFilter) throws Exception {
        http
            .csrf(AbstractHttpConfigurer::disable)
            .addFilterBefore(callerIdFilter, UsernamePasswordAuthenticationFilter.class)
            .authorizeHttpRequests(auth -> auth.anyRequest().permitAll());
        return http.build();
    }
}
