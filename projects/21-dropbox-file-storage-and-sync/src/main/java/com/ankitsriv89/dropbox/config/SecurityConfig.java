package com.ankitsriv89.dropbox.config;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.web.filter.OncePerRequestFilter;

import java.io.IOException;
import java.util.regex.Pattern;

/*
 * Demo auth guard.
 *
 * Every request to /v1/** must carry a valid X-Api-Key header. The key is
 * configured via the API_KEY env var and is shared across all demo users.
 *
 * X-Owner-Id is still caller-supplied, which is fine for a single-tenant demo
 * where the shared key proves authorization. In production, replace this with
 * JWT/OAuth2: derive ownerId from the authenticated principal
 * (@AuthenticationPrincipal), never trust a client-supplied identity header.
 */
@Configuration
public class SecurityConfig {

    private static final Pattern SAFE_OWNER_ID = Pattern.compile("^[\\w\\-]{1,64}$");

    @Bean
    public OncePerRequestFilter apiKeyFilter(@Value("${api.key:demo-secret}") String configuredKey) {
        return new OncePerRequestFilter() {
            @Override
            protected void doFilterInternal(HttpServletRequest req,
                                            HttpServletResponse res,
                                            FilterChain chain) throws ServletException, IOException {
                String path = req.getRequestURI();

                // Let health, actuator, and static assets through unauthenticated
                if (!path.startsWith("/v1/") || path.equals("/v1/health")) {
                    chain.doFilter(req, res);
                    return;
                }

                // Validate API key
                String key = req.getHeader("X-Api-Key");
                if (key == null || !key.equals(configuredKey)) {
                    res.setStatus(HttpServletResponse.SC_UNAUTHORIZED);
                    res.setContentType("application/json");
                    res.getWriter().write("{\"error\":\"Missing or invalid X-Api-Key\"}");
                    return;
                }

                // Validate owner-id shape to prevent path-traversal style abuse
                String ownerId = req.getHeader("X-Owner-Id");
                if (ownerId == null || !SAFE_OWNER_ID.matcher(ownerId).matches()) {
                    res.setStatus(HttpServletResponse.SC_BAD_REQUEST);
                    res.setContentType("application/json");
                    res.getWriter().write("{\"error\":\"Missing or invalid X-Owner-Id (alphanumeric + hyphens, max 64 chars)\"}");
                    return;
                }

                chain.doFilter(req, res);
            }
        };
    }
}
