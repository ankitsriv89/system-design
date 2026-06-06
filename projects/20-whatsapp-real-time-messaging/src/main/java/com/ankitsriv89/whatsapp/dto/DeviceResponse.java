package com.ankitsriv89.whatsapp.dto;

import com.ankitsriv89.whatsapp.domain.Device;
import java.time.Instant;

public record DeviceResponse(Long id, Long userId, String publicKey, String label, Instant lastSeen) {
    public static DeviceResponse from(Device d) {
        return new DeviceResponse(d.getId(), d.getUser().getId(), d.getPublicKey(), d.getLabel(), d.getLastSeen());
    }
}
