package com.ankitsriv89.whatsapp.repository;

import com.ankitsriv89.whatsapp.domain.Device;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import java.util.List;

public interface DeviceRepository extends JpaRepository<Device, Long> {
    List<Device> findByUserId(Long userId);

    @Query("SELECT d FROM Device d WHERE d.user.username = :username")
    List<Device> findByUsername(String username);
}
