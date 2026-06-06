package com.ankitsriv89.instagram.store;

import io.minio.GetObjectArgs;
import io.minio.GetPresignedObjectUrlArgs;
import io.minio.MinioClient;
import io.minio.PutObjectArgs;
import io.minio.StatObjectArgs;
import io.minio.errors.ErrorResponseException;
import io.minio.http.Method;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import java.io.ByteArrayInputStream;
import java.io.InputStream;
import java.util.concurrent.TimeUnit;

/**
 * Low-level adapter over the S3-compatible object store (MinIO). Issues
 * presigned PUT URLs so clients upload bytes <em>directly</em> to storage,
 * keeping large media off the application's request path.
 */
@Component
public class MediaStore {

    private final MinioClient client;
    private final String bucket;
    private final int presignExpirySeconds;

    public MediaStore(MinioClient client,
                      @Value("${instagram.media.minio.bucket}") String bucket,
                      @Value("${instagram.media.presign-expiry-seconds:900}") int presignExpirySeconds) {
        this.client = client;
        this.bucket = bucket;
        this.presignExpirySeconds = presignExpirySeconds;
    }

    /** Presigned PUT URL the client uses to upload the original bytes. */
    public String presignedPutUrl(String objectKey) {
        try {
            return client.getPresignedObjectUrl(
                    GetPresignedObjectUrlArgs.builder()
                            .method(Method.PUT)
                            .bucket(bucket)
                            .object(objectKey)
                            .expiry(presignExpirySeconds, TimeUnit.SECONDS)
                            .build());
        } catch (Exception e) {
            throw new IllegalStateException("Failed to presign PUT for " + objectKey, e);
        }
    }

    /** Download an object's full bytes (used by the variant worker). */
    public byte[] getObject(String objectKey) {
        try (InputStream in = client.getObject(
                GetObjectArgs.builder().bucket(bucket).object(objectKey).build())) {
            return in.readAllBytes();
        } catch (Exception e) {
            throw new IllegalStateException("Failed to read " + objectKey, e);
        }
    }

    /** Upload a generated variant's bytes. */
    public void putObject(String objectKey, byte[] data, String contentType) {
        try (ByteArrayInputStream in = new ByteArrayInputStream(data)) {
            client.putObject(PutObjectArgs.builder()
                    .bucket(bucket)
                    .object(objectKey)
                    .stream(in, data.length, -1)
                    .contentType(contentType)
                    .build());
        } catch (Exception e) {
            throw new IllegalStateException("Failed to write " + objectKey, e);
        }
    }

    /** Whether the object actually exists in storage (used to confirm uploads). */
    public boolean objectExists(String objectKey) {
        try {
            client.statObject(StatObjectArgs.builder().bucket(bucket).object(objectKey).build());
            return true;
        } catch (ErrorResponseException e) {
            return false;
        } catch (Exception e) {
            throw new IllegalStateException("Failed to stat " + objectKey, e);
        }
    }
}
