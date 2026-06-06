package com.ankitsriv89.instagram;

import com.ankitsriv89.instagram.domain.Media;
import com.ankitsriv89.instagram.dto.CreateUploadResponse;
import com.ankitsriv89.instagram.dto.MediaUploadedEvent;
import com.ankitsriv89.instagram.repository.MediaRepository;
import com.ankitsriv89.instagram.service.MediaService;
import com.ankitsriv89.instagram.store.EventPublisher;
import com.ankitsriv89.instagram.store.MediaStore;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.Optional;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.*;

/**
 * Milestone 1 acceptance — upload + metadata flow, mocked storage/Kafka.
 */
@ExtendWith(MockitoExtension.class)
class InstagramApplicationTests {

    @Mock MediaRepository mediaRepository;
    @Mock MediaStore mediaStore;
    @Mock EventPublisher eventPublisher;
    @InjectMocks MediaService mediaService;

    @Test
    void beginUpload_createsPendingRecordAndPresignsUrl() {
        when(mediaRepository.save(any(Media.class))).thenAnswer(inv -> inv.getArgument(0));
        when(mediaStore.presignedPutUrl(anyString())).thenReturn("http://minio/presigned");

        CreateUploadResponse res = mediaService.beginUpload(42L, "image/jpeg");

        assertThat(res.uploadUrl()).isEqualTo("http://minio/presigned");
        assertThat(res.objectKey()).startsWith("originals/42/");
        verify(mediaRepository).save(any(Media.class));
    }

    @Test
    void completeUpload_emitsMediaUploadedEvent_whenObjectPresent() {
        Media media = new Media(42L, "originals/42/abc", "image/jpeg");
        when(mediaRepository.findById(1L)).thenReturn(Optional.of(media));
        when(mediaStore.objectExists("originals/42/abc")).thenReturn(true);

        mediaService.completeUpload(1L, 42L);

        ArgumentCaptor<MediaUploadedEvent> captor = ArgumentCaptor.forClass(MediaUploadedEvent.class);
        verify(eventPublisher).publishMediaUploaded(captor.capture());
        assertThat(captor.getValue().objectKey()).isEqualTo("originals/42/abc");
    }

    @Test
    void completeUpload_rejectsWrongOwner() {
        Media media = new Media(42L, "originals/42/abc", "image/jpeg");
        when(mediaRepository.findById(1L)).thenReturn(Optional.of(media));

        assertThatThrownBy(() -> mediaService.completeUpload(1L, 99L))
                .isInstanceOf(SecurityException.class);
        verifyNoInteractions(eventPublisher);
    }

    @Test
    void completeUpload_failsWhenObjectMissing() {
        Media media = new Media(42L, "originals/42/abc", "image/jpeg");
        when(mediaRepository.findById(1L)).thenReturn(Optional.of(media));
        when(mediaStore.objectExists("originals/42/abc")).thenReturn(false);

        assertThatThrownBy(() -> mediaService.completeUpload(1L, 42L))
                .isInstanceOf(IllegalStateException.class);
        verifyNoInteractions(eventPublisher);
    }
}
