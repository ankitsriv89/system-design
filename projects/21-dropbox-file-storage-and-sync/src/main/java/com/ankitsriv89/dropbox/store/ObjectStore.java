package com.ankitsriv89.dropbox.store;

import lombok.RequiredArgsConstructor;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;
import software.amazon.awssdk.core.sync.RequestBody;
import software.amazon.awssdk.services.s3.S3Client;
import software.amazon.awssdk.services.s3.model.*;
import java.io.InputStream;

@Component
@RequiredArgsConstructor
public class ObjectStore {
    private final S3Client s3;

    @Value("${s3.bucket}")
    private String bucket;

    public void ensureBucket() {
        try {
            s3.headBucket(r -> r.bucket(bucket));
        } catch (NoSuchBucketException e) {
            s3.createBucket(r -> r.bucket(bucket));
        }
    }

    public void put(String key, InputStream data, long contentLength, String contentType) {
        s3.putObject(
                PutObjectRequest.builder().bucket(bucket).key(key).contentType(contentType).build(),
                RequestBody.fromInputStream(data, contentLength));
    }

    public InputStream get(String key) {
        return s3.getObject(GetObjectRequest.builder().bucket(bucket).key(key).build());
    }

    public void delete(String key) {
        s3.deleteObject(DeleteObjectRequest.builder().bucket(bucket).key(key).build());
    }

    public String presignedUrl(String key) {
        // Presigned URL generation requires S3Presigner — stub returns path for now
        return "/v1/files/download/" + key;
    }
}
