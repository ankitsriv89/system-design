package com.ankitsriv89.chat;

import com.ankitsriv89.chat.config.JwtUtil;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class JwtUtilTest {

    private final JwtUtil jwt = new JwtUtil("test-secret-that-is-at-least-32-bytes!!", 60);

    @Test
    void roundTrip() {
        String token = jwt.generate("alice");
        assertEquals("alice", jwt.validate(token));
    }

    @Test
    void invalidToken_throws() {
        assertThrows(Exception.class, () -> jwt.validate("not.a.jwt"));
    }

    @Test
    void differentUserIds() {
        String t1 = jwt.generate("alice");
        String t2 = jwt.generate("bob");
        assertEquals("alice", jwt.validate(t1));
        assertEquals("bob", jwt.validate(t2));
        assertNotEquals(t1, t2);
    }
}
