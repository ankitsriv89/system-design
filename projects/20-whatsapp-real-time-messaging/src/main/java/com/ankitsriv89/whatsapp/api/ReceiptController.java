package com.ankitsriv89.whatsapp.api;

import com.ankitsriv89.whatsapp.dto.ReceiptUpdateRequest;
import com.ankitsriv89.whatsapp.service.ReceiptService;
import jakarta.validation.Valid;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.Authentication;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/v1/receipts")
public class ReceiptController {

    private final ReceiptService receiptService;

    public ReceiptController(ReceiptService receiptService) { this.receiptService = receiptService; }

    @PostMapping
    public ResponseEntity<Void> update(
            Authentication auth,
            @Valid @RequestBody ReceiptUpdateRequest req,
            @RequestParam Long deviceId) {
        receiptService.advance(auth.getName(), req.messageId(), deviceId, req.state());
        return ResponseEntity.noContent().build();
    }
}
