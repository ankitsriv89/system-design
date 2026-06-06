package com.ankitsriv89.whatsapp.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;

public record SendMessageRequest(
        @NotBlank String chatId,
        // Base64-encoded ciphertext; the server does not decrypt this.
        @NotNull String ciphertext
) {}
