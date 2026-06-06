package com.ankitsriv89.instagram.config;

import io.minio.BucketExistsArgs;
import io.minio.MakeBucketArgs;
import io.minio.MinioClient;
import jakarta.annotation.PostConstruct;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * MinIO client wiring. MinIO is the S3-compatible <em>origin</em> for media
 * bytes; in production a Cloudflare CDN sits in front of {@code public-base-url}.
 *
 * <p>The bucket is created on startup if absent so the demo is self-contained.
 */
@Configuration
public class MinioConfig {

    private static final Logger log = LoggerFactory.getLogger(MinioConfig.class);

    @Value("${instagram.media.minio.endpoint}")
    private String endpoint;

    @Value("${instagram.media.minio.access-key}")
    private String accessKey;

    @Value("${instagram.media.minio.secret-key}")
    private String secretKey;

    @Value("${instagram.media.minio.bucket}")
    private String bucket;

    @Bean
    public MinioClient minioClient() {
        return MinioClient.builder()
                .endpoint(endpoint)
                .credentials(accessKey, secretKey)
                .build();
    }

    @PostConstruct
    public void ensureBucket() throws Exception {
        MinioClient client = minioClient();
        boolean exists = client.bucketExists(BucketExistsArgs.builder().bucket(bucket).build());
        if (!exists) {
            client.makeBucket(MakeBucketArgs.builder().bucket(bucket).build());
            log.info("Created MinIO bucket '{}'", bucket);
        } else {
            log.info("MinIO bucket '{}' already present", bucket);
        }
    }
}
