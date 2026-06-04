package com.ankitsriv89.groupchat.dto;

public class KafkaMessageEvent {
    private Long messageId;
    private Long groupId;
    private String senderId;
    private String body;
    private Long seq;

    public KafkaMessageEvent() {}

    public KafkaMessageEvent(Long messageId, Long groupId, String senderId, String body, Long seq) {
        this.messageId = messageId;
        this.groupId = groupId;
        this.senderId = senderId;
        this.body = body;
        this.seq = seq;
    }

    public Long getMessageId() { return messageId; }
    public Long getGroupId() { return groupId; }
    public String getSenderId() { return senderId; }
    public String getBody() { return body; }
    public Long getSeq() { return seq; }
}
