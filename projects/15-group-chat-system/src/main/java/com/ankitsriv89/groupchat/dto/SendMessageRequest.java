package com.ankitsriv89.groupchat.dto;

public class SendMessageRequest {
    private Long groupId;
    private String body;

    public Long getGroupId() { return groupId; }
    public void setGroupId(Long groupId) { this.groupId = groupId; }
    public String getBody() { return body; }
    public void setBody(String body) { this.body = body; }
}
