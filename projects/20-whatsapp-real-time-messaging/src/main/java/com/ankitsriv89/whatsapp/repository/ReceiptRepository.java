package com.ankitsriv89.whatsapp.repository;

import com.ankitsriv89.whatsapp.domain.Receipt;
import com.ankitsriv89.whatsapp.domain.ReceiptId;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import java.util.List;

public interface ReceiptRepository extends JpaRepository<Receipt, ReceiptId> {

    @Query("SELECT r FROM Receipt r WHERE r.device.id = :deviceId AND r.state = 'SENT'")
    List<Receipt> findPendingForDevice(Long deviceId);

    List<Receipt> findByMessageId(Long messageId);
}
