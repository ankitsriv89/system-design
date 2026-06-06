package com.ankitsriv89.whatsapp.service;

import com.ankitsriv89.whatsapp.domain.Device;
import com.ankitsriv89.whatsapp.dto.DeviceRegisterRequest;
import com.ankitsriv89.whatsapp.dto.DeviceResponse;
import com.ankitsriv89.whatsapp.repository.DeviceRepository;
import com.ankitsriv89.whatsapp.repository.UserRepository;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;

@Service
public class DeviceService {

    private final DeviceRepository devices;
    private final UserRepository users;

    public DeviceService(DeviceRepository devices, UserRepository users) {
        this.devices = devices;
        this.users = users;
    }

    @Transactional
    public DeviceResponse register(String username, DeviceRegisterRequest req) {
        var user = users.findByUsername(username)
                .orElseThrow(() -> new IllegalStateException("User not found: " + username));
        Device device = devices.save(new Device(user, req.publicKey(), req.label()));
        return DeviceResponse.from(device);
    }

    public List<DeviceResponse> listForUser(String username) {
        return devices.findByUsername(username).stream().map(DeviceResponse::from).toList();
    }

    public List<Device> devicesForUser(Long userId) {
        return devices.findByUserId(userId);
    }
}
