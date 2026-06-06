package com.ankitsriv89.whatsapp.api;

import com.ankitsriv89.whatsapp.dto.DeviceRegisterRequest;
import com.ankitsriv89.whatsapp.dto.DeviceResponse;
import com.ankitsriv89.whatsapp.service.DeviceService;
import jakarta.validation.Valid;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.Authentication;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/v1/devices")
public class DeviceController {

    private final DeviceService deviceService;

    public DeviceController(DeviceService deviceService) { this.deviceService = deviceService; }

    @PostMapping
    public ResponseEntity<DeviceResponse> register(
            Authentication auth,
            @Valid @RequestBody DeviceRegisterRequest req) {
        return ResponseEntity.ok(deviceService.register(auth.getName(), req));
    }

    @GetMapping
    public ResponseEntity<List<DeviceResponse>> list(Authentication auth) {
        return ResponseEntity.ok(deviceService.listForUser(auth.getName()));
    }
}
