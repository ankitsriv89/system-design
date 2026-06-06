package com.ankitsriv89.whatsapp.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public record RegisterRequest(
        @NotBlank @Size(min = 2, max = 40) String username,
        @NotBlank @Size(min = 8) String password
) {}
