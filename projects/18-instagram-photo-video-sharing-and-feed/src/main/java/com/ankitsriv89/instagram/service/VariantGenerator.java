package com.ankitsriv89.instagram.service;

import net.coobird.thumbnailator.Thumbnails;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.util.LinkedHashMap;
import java.util.Map;

/**
 * Generates resized image variants (thumbnail/small/medium) from original bytes.
 * Pure image work via Thumbnailator (no native deps) so it is unit-testable in
 * isolation from storage and Kafka.
 */
@Component
public class VariantGenerator {

    private final int thumbnailPx;
    private final int smallPx;
    private final int mediumPx;

    public VariantGenerator(
            @Value("${instagram.media.variants.thumbnail-px}") int thumbnailPx,
            @Value("${instagram.media.variants.small-px}") int smallPx,
            @Value("${instagram.media.variants.medium-px}") int mediumPx) {
        this.thumbnailPx = thumbnailPx;
        this.smallPx = smallPx;
        this.mediumPx = mediumPx;
    }

    public boolean isImage(String contentType) {
        return contentType != null && contentType.startsWith("image/");
    }

    /**
     * Returns variant-name -> JPEG bytes. Each variant fits within an N×N box,
     * preserving aspect ratio. Throws if the bytes are not a decodable image.
     */
    public Map<String, byte[]> generate(byte[] original) {
        Map<String, byte[]> out = new LinkedHashMap<>();
        out.put("thumbnail", resize(original, thumbnailPx));
        out.put("small", resize(original, smallPx));
        out.put("medium", resize(original, mediumPx));
        return out;
    }

    private byte[] resize(byte[] original, int box) {
        try (ByteArrayInputStream in = new ByteArrayInputStream(original);
             ByteArrayOutputStream out = new ByteArrayOutputStream()) {
            Thumbnails.of(in)
                    .size(box, box)
                    .keepAspectRatio(true)
                    .outputFormat("jpg")
                    .toOutputStream(out);
            return out.toByteArray();
        } catch (Exception e) {
            throw new IllegalArgumentException("Failed to resize image to " + box + "px", e);
        }
    }
}
