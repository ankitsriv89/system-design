package com.ankitsriv89.instagram;

import com.ankitsriv89.instagram.service.VariantGenerator;
import org.junit.jupiter.api.Test;

import javax.imageio.ImageIO;
import java.awt.Graphics2D;
import java.awt.image.BufferedImage;
import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

/**
 * Milestone 2 acceptance — real image variant generation.
 */
class VariantGeneratorTest {

    // thumbnail=150, small=320, medium=640 (matches application.yml)
    private final VariantGenerator generator = new VariantGenerator(150, 320, 640);

    private static byte[] sampleImage(int w, int h) throws Exception {
        BufferedImage img = new BufferedImage(w, h, BufferedImage.TYPE_INT_RGB);
        Graphics2D g = img.createGraphics();
        g.fillRect(0, 0, w, h);
        g.dispose();
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        ImageIO.write(img, "jpg", out);
        return out.toByteArray();
    }

    @Test
    void generatesThreeDecodableVariants() throws Exception {
        byte[] original = sampleImage(1000, 800);

        Map<String, byte[]> variants = generator.generate(original);

        assertThat(variants).containsOnlyKeys("thumbnail", "small", "medium");
        for (var entry : variants.entrySet()) {
            BufferedImage decoded = ImageIO.read(new ByteArrayInputStream(entry.getValue()));
            assertThat(decoded).as("variant %s decodes", entry.getKey()).isNotNull();
            // longest side must fit within the requested box
            int box = switch (entry.getKey()) {
                case "thumbnail" -> 150;
                case "small" -> 320;
                default -> 640;
            };
            assertThat(Math.max(decoded.getWidth(), decoded.getHeight())).isLessThanOrEqualTo(box);
        }
    }

    @Test
    void isImage_discriminatesContentType() {
        assertThat(generator.isImage("image/jpeg")).isTrue();
        assertThat(generator.isImage("image/png")).isTrue();
        assertThat(generator.isImage("video/mp4")).isFalse();
        assertThat(generator.isImage(null)).isFalse();
    }

    @Test
    void rejectsNonImageBytes() {
        assertThatThrownBy(() -> generator.generate("not an image".getBytes()))
                .isInstanceOf(IllegalArgumentException.class);
    }
}
