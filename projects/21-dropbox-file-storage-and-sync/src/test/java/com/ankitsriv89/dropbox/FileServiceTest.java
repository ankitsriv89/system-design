package com.ankitsriv89.dropbox;

import com.ankitsriv89.dropbox.domain.FileNode;
import com.ankitsriv89.dropbox.repository.FileNodeRepository;
import com.ankitsriv89.dropbox.repository.FileVersionRepository;
import com.ankitsriv89.dropbox.repository.ChangeEventRepository;
import com.ankitsriv89.dropbox.service.FileService;
import com.ankitsriv89.dropbox.store.ObjectStore;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.kafka.core.KafkaTemplate;

import java.util.List;
import java.util.Optional;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

@ExtendWith(MockitoExtension.class)
class FileServiceTest {

    @Mock FileNodeRepository fileNodeRepo;
    @Mock FileVersionRepository fileVersionRepo;
    @Mock ChangeEventRepository changeEventRepo;
    @Mock ObjectStore objectStore;
    @Mock KafkaTemplate<String, Object> kafkaTemplate;

    @InjectMocks FileService fileService;

    @Test
    void createFolder_persistsNodeAndPublishesEvent() {
        FileNode saved = new FileNode();
        saved.setId(UUID.randomUUID());
        saved.setName("Documents");
        saved.setType(FileNode.NodeType.FOLDER);
        when(fileNodeRepo.save(any())).thenReturn(saved);
        when(changeEventRepo.save(any())).thenReturn(null);

        FileNode result = fileService.createFolder("user1", null, "Documents");

        assertThat(result.getName()).isEqualTo("Documents");
        assertThat(result.getType()).isEqualTo(FileNode.NodeType.FOLDER);
        verify(fileNodeRepo).save(any());
        verify(changeEventRepo).save(any());
        verify(kafkaTemplate).send(eq("dropbox.file-events"), eq("user1"), any());
    }

    @Test
    void listFolder_returnsEmptyWhenNoChildren() {
        when(fileNodeRepo.findByOwnerIdAndParentIdAndDeletedAtIsNull("user1", null))
                .thenReturn(List.of());

        List<FileNode> result = fileService.listFolder("user1", null);
        assertThat(result).isEmpty();
    }

    @Test
    void getFile_throwsWhenNotFound() {
        UUID id = UUID.randomUUID();
        when(fileNodeRepo.findByIdAndOwnerIdAndDeletedAtIsNull(id, "user1"))
                .thenReturn(Optional.empty());

        org.junit.jupiter.api.Assertions.assertThrows(
                java.util.NoSuchElementException.class,
                () -> fileService.getFile("user1", id));
    }
}
