package com.ankitsriv89.groupchat.dto;

public class WsEnvelope {
    private String type;     // message, join, leave, history, error
    private Long groupId;
    private String senderId;
    private String body;
    private Long seq;
    private Object data;

    public WsEnvelope() {}

    public String getType() { return type; }
    public void setType(String type) { this.type = type; }
    public Long getGroupId() { return groupId; }
    public void setGroupId(Long groupId) { this.groupId = groupId; }
    public String getSenderId() { return senderId; }
    public void setSenderId(String senderId) { this.senderId = senderId; }
    public String getBody() { return body; }
    public void setBody(String body) { this.body = body; }
    public Long getSeq() { return seq; }
    public void setSeq(Long seq) { this.seq = seq; }
    public Object getData() { return data; }
    public void setData(Object data) { this.data = data; }
}
