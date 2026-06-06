package com.ankitsriv89.instagram;

import com.ankitsriv89.instagram.service.MediaUrlService;
import org.junit.jupiter.api.Test;

import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * Milestone 4 acceptance — the CDN swap point: object keys become public URLs,
 * and the same code works for the local base path and a Cloudflare host.
 */
class MediaUrlServiceTest {

    @Test
    void buildsLocalUrls() {
        MediaUrlService svc = new MediaUrlService("/p18/media");
        assertThat(svc.urlFor("variants/42/thumbnail.jpg"))
                .isEqualTo("/p18/media/variants/42/thumbnail.jpg");
    }

    @Test
    void buildsCdnUrls_andTrimsTrailingSlash() {
        MediaUrlService svc = new MediaUrlService("https://cdn.example.com/");
        assertThat(svc.urlFor("variants/42/medium.jpg"))
                .isEqualTo("https://cdn.example.com/variants/42/medium.jpg");
    }

    @Test
    void mapsVariantKeysToUrls() {
        MediaUrlService svc = new MediaUrlService("https://cdn.example.com");
        Map<String, String> urls = svc.urlsFor(Map.of(
                "thumbnail", "variants/1/thumbnail.jpg",
                "original", "originals/1/abc"));
        assertThat(urls).containsEntry("thumbnail", "https://cdn.example.com/variants/1/thumbnail.jpg");
        assertThat(urls).containsEntry("original", "https://cdn.example.com/originals/1/abc");
    }
}
