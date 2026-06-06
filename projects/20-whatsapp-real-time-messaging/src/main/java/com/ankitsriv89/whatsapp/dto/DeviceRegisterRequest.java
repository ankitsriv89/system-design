package com.ankitsriv89.whatsapp.dto;

import jakarta.validation.constraints.NotBlank;

public record DeviceRegisterRequest(
        @NotBlank String publicKey,
        String label
) {}
