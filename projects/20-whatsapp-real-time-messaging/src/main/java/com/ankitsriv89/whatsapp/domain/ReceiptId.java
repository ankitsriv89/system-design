package com.ankitsriv89.whatsapp.domain;

import jakarta.persistence.Embeddable;
import java.io.Serializable;
import java.util.Objects;

@Embeddable
public class ReceiptId implements Serializable {

    private Long messageId;
    private Long deviceId;

    protected ReceiptId() {}

    public ReceiptId(Long messageId, Long deviceId) {
        this.messageId = messageId;
        this.deviceId = deviceId;
    }

    public Long getMessageId() { return messageId; }
    public Long getDeviceId() { return deviceId; }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (!(o instanceof ReceiptId r)) return false;
        return Objects.equals(messageId, r.messageId) && Objects.equals(deviceId, r.deviceId);
    }

    @Override
    public int hashCode() { return Objects.hash(messageId, deviceId); }
}
