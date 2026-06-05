package com.ankitsriv89.proximityservice.domain;

/**
 * Minimal geohash encoder (Base32). Precision 9 ~ 4.8m accuracy.
 * Uses the standard geohash alphabet: 0-9 and b-z (no a, i, l, o).
 */
public final class GeohashUtil {

    private static final String BASE32 = "0123456789bcdefghjkmnpqrstuvwxyz";
    private static final int DEFAULT_PRECISION = 9;

    private GeohashUtil() {}

    public static String encode(double lat, double lon) {
        return encode(lat, lon, DEFAULT_PRECISION);
    }

    public static String encode(double lat, double lon, int precision) {
        double minLat = -90, maxLat = 90;
        double minLon = -180, maxLon = 180;

        StringBuilder hash = new StringBuilder();
        int bits = 0, bitsTotal = 0, hashValue = 0;
        boolean isLon = true;

        while (hash.length() < precision) {
            if (isLon) {
                double mid = (minLon + maxLon) / 2;
                if (lon > mid) { hashValue = (hashValue << 1) | 1; minLon = mid; }
                else           { hashValue = hashValue << 1;          maxLon = mid; }
            } else {
                double mid = (minLat + maxLat) / 2;
                if (lat > mid) { hashValue = (hashValue << 1) | 1; minLat = mid; }
                else           { hashValue = hashValue << 1;          maxLat = mid; }
            }
            isLon = !isLon;
            bits++;
            bitsTotal++;
            if (bits == 5) {
                hash.append(BASE32.charAt(hashValue));
                bits = 0;
                hashValue = 0;
            }
        }
        return hash.toString();
    }

    /** Returns the 8 neighbours + center for a geohash prefix. */
    public static String[] neighbors(String hash) {
        // simplified: return only center for MVP; expand for production
        return new String[]{ hash };
    }
}
