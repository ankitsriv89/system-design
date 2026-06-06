package com.ankitsriv89.whatsapp.dto;

import jakarta.validation.constraints.NotBlank;
import java.util.List;

public record GroupCreateRequest(
        @NotBlank String name,
        List<Long> memberUserIds
) {}
