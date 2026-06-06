package com.ankitsriv89.instagram.service;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import java.util.LinkedHashMap;
import java.util.Map;

/**
 * Milestone 4 — builds the <em>public</em> URL for a media object from its
 * object key. This is the single swap point between environments:
 *
 * <ul>
 *   <li>MVP / local: {@code public-base-url = /p18/media}, served by
 *       {@link com.ankitsriv89.instagram.api.MediaObjectController} straight from
 *       MinIO with cache headers.</li>
 *   <li>Production: {@code public-base-url = https://cdn.example.com}, a
 *       Cloudflare zone whose origin is MinIO. The application code is identical;
 *       only the config value changes.</li>
 * </ul>
 */
@Service
public class MediaUrlService {

    private final String publicBaseUrl;

    public MediaUrlService(@Value("${instagram.media.public-base-url}") String publicBaseUrl) {
        // normalize: no trailing slash
        this.publicBaseUrl = publicBaseUrl.endsWith("/")
                ? publicBaseUrl.substring(0, publicBaseUrl.length() - 1)
                : publicBaseUrl;
    }

    /** Public URL for a single object key. */
    public String urlFor(String objectKey) {
        return publicBaseUrl + "/" + objectKey;
    }

    /** Map variant-name -> public URL for a media's variant key map. */
    public Map<String, String> urlsFor(Map<String, String> variantKeys) {
        Map<String, String> out = new LinkedHashMap<>();
        variantKeys.forEach((name, key) -> out.put(name, urlFor(key)));
        return out;
    }
}
