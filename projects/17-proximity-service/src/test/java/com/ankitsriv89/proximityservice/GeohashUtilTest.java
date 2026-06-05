package com.ankitsriv89.proximityservice;

import com.ankitsriv89.proximityservice.domain.GeohashUtil;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class GeohashUtilTest {

    @Test
    void encodeProducesCorrectLength() {
        String hash = GeohashUtil.encode(37.7749, -122.4194);
        assertEquals(9, hash.length());
    }

    @Test
    void encodeSanFranciscoStartsWithCorrectPrefix() {
        String hash = GeohashUtil.encode(37.7749, -122.4194);
        assertTrue(hash.startsWith("9q8y"), "SF geohash should start with 9q8y, got: " + hash);
    }

    @Test
    void encodeTokyo() {
        String hash = GeohashUtil.encode(35.6762, 139.6503);
        assertTrue(hash.startsWith("xn7"), "Tokyo geohash should start with xn7, got: " + hash);
    }
}
