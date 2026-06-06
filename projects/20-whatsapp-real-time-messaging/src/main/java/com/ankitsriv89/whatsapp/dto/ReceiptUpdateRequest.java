package com.ankitsriv89.whatsapp.dto;

import jakarta.validation.constraints.NotNull;

public record ReceiptUpdateRequest(
        @NotNull Long messageId,
        @NotNull String state
) {}
