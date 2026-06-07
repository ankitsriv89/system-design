package com.ankitsriv89.ticketbooking.api;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import org.springframework.stereotype.Component;
import org.springframework.web.filter.OncePerRequestFilter;

import java.io.IOException;

/**
 * Rejects /v1/** requests that arrive without an X-User-Id header.
 *
 * This is a minimal guard against accidental unauthenticated calls, not a
 * cryptographic proof of identity. The header value itself is trusted on the
 * assumption that traffic reaching this port has already passed through the
 * auth gateway. TODO: replace with OAuth2 JWT resource-server validation.
 */
@Component
public class CallerIdFilter extends OncePerRequestFilter {

    @Override
    protected void doFilterInternal(HttpServletRequest request,
                                    HttpServletResponse response,
                                    FilterChain chain) throws ServletException, IOException {
        String path = request.getRequestURI();
        if (path.startsWith("/v1/")) {
            String callerId = request.getHeader("X-User-Id");
            if (callerId == null || callerId.isBlank()) {
                response.setStatus(HttpServletResponse.SC_UNAUTHORIZED);
                response.setContentType("application/json");
                response.getWriter().write("{\"error\":\"X-User-Id header required\"}");
                return;
            }
        }
        chain.doFilter(request, response);
    }
}
