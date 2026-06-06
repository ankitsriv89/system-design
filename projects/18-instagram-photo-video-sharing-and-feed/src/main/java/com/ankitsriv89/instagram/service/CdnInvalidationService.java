package com.ankitsriv89.instagram.service;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;

import java.util.Collection;

/**
 * Milestone 4 — cache invalidation.
 *
 * <p>When a media object's bytes change at the same key (reprocess) or a post is
 * deleted, any cached copy at the edge must be purged or it will serve stale
 * content (the "deleted media cached" failure mode in plan.md).
 *
 * <p>This service is the single invalidation seam. Locally it logs the purge
 * (there is no shared edge cache to evict beyond the browser's, which the
 * Cache-Control headers govern). In production this is where a Cloudflare purge
 * call goes:
 * <pre>
 *   POST https://api.cloudflare.com/client/v4/zones/{zoneId}/purge_cache
 *   { "files": ["https://cdn.example.com/variants/42/medium.jpg", ...] }
 * </pre>
 * Keying invalidation by exact URL keeps purges cheap and targeted.
 */
@Service
public class CdnInvalidationService {

    private static final Logger log = LoggerFactory.getLogger(CdnInvalidationService.class);

    private final MediaUrlService urls;

    public CdnInvalidationService(MediaUrlService urls) {
        this.urls = urls;
    }

    /** Purge a set of object keys from the edge cache. */
    public void purge(Collection<String> objectKeys) {
        if (objectKeys.isEmpty()) {
            return;
        }
        var purgedUrls = objectKeys.stream().map(urls::urlFor).toList();
        // Production: issue Cloudflare purge_cache here. Local: nothing to evict.
        log.info("cdn.purge count={} urls={}", purgedUrls.size(), purgedUrls);
    }
}
