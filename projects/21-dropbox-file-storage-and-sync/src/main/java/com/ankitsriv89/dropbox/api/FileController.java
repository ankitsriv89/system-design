package com.ankitsriv89.dropbox.api;

import com.ankitsriv89.dropbox.domain.FileNode;
import com.ankitsriv89.dropbox.service.FileService;
import com.ankitsriv89.dropbox.service.SyncService;
import lombok.RequiredArgsConstructor;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.multipart.MultipartFile;

import java.io.IOException;
import java.util.List;
import java.util.Map;
import java.util.UUID;

@RestController
@RequestMapping("/v1")
@RequiredArgsConstructor
public class FileController {
    private final FileService fileService;
    private final SyncService syncService;

    // List folder contents
    @GetMapping("/folders")
    public List<FileNode> listRoot(@RequestHeader("X-Owner-Id") String ownerId) {
        return fileService.listFolder(ownerId, null);
    }

    @GetMapping("/folders/{folderId}/children")
    public List<FileNode> listFolder(
            @RequestHeader("X-Owner-Id") String ownerId,
            @PathVariable UUID folderId) {
        return fileService.listFolder(ownerId, folderId);
    }

    // Create folder
    @PostMapping("/folders")
    public FileNode createFolder(
            @RequestHeader("X-Owner-Id") String ownerId,
            @RequestParam(required = false) UUID parentId,
            @RequestParam String name) {
        return fileService.createFolder(ownerId, parentId, name);
    }

    // Upload file (multipart)
    @PostMapping("/files")
    public FileNode uploadFile(
            @RequestHeader("X-Owner-Id") String ownerId,
            @RequestParam(required = false) UUID parentId,
            @RequestParam("file") MultipartFile file) throws IOException {
        return fileService.uploadFile(ownerId, parentId, file);
    }

    // Get file metadata
    @GetMapping("/files/{fileId}")
    public FileNode getFile(
            @RequestHeader("X-Owner-Id") String ownerId,
            @PathVariable UUID fileId) {
        return fileService.getFile(ownerId, fileId);
    }

    // Soft-delete
    @DeleteMapping("/files/{fileId}")
    public ResponseEntity<Void> deleteFile(
            @RequestHeader("X-Owner-Id") String ownerId,
            @PathVariable UUID fileId) {
        fileService.deleteFile(ownerId, fileId);
        return ResponseEntity.noContent().build();
    }

    // Sync poll — returns change events since cursor
    @GetMapping("/sync")
    public Map<String, Object> sync(
            @RequestHeader("X-Owner-Id") String ownerId,
            @RequestHeader("X-Device-Id") String deviceId,
            @RequestParam(defaultValue = "0") long cursor) {
        SyncService.SyncResult result = syncService.poll(ownerId, deviceId);
        return Map.of(
                "events", result.events(),
                "previousCursor", result.previousCursor(),
                "newCursor", result.events().isEmpty() ? cursor
                        : result.events().getLast().getId()
        );
    }

    // Health/info
    @GetMapping("/health")
    public Map<String, String> health() {
        return Map.of("status", "ok", "service", "dropbox-file-storage-and-sync");
    }
}
