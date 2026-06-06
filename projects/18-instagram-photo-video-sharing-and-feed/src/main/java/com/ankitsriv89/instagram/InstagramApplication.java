package com.ankitsriv89.instagram;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

/**
 * Project 18 — Instagram Photo/Video Sharing and Feed.
 *
 * <p>Media bytes flow to an S3-compatible object store (MinIO) and are served
 * via a CDN (Cloudflare) in production. Metadata, the social graph, and
 * engagement live in Postgres; hot feeds and counters live in Redis. Kafka
 * separates upload acceptance from variant processing and feed fanout.
 */
@SpringBootApplication
public class InstagramApplication {

    public static void main(String[] args) {
        SpringApplication.run(InstagramApplication.class, args);
    }
}
