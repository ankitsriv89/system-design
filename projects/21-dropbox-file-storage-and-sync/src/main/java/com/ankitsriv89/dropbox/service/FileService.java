package com.ankitsriv89.dropbox.service;

import com.ankitsriv89.dropbox.domain.*;
import com.ankitsriv89.dropbox.repository.*;
import com.ankitsriv89.dropbox.store.ObjectStore;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.web.multipart.MultipartFile;

import java.io.IOException;
import java.security.MessageDigest;
import java.time.Instant;
import java.util.*;

@Slf4j
@Service
@RequiredArgsConstructor
public class FileService {
    private final FileNodeRepository fileNodeRepo;
    private final FileVersionRepository fileVersionRepo;
    private final ChangeEventRepository changeEventRepo;
    private final ObjectStore objectStore;
    private final KafkaTemplate<String, Object> kafkaTemplate;

    @Transactional
    public FileNode createFolder(String ownerId, UUID parentId, String name) {
        FileNode node = new FileNode();
        node.setOwnerId(ownerId);
        node.setParentId(parentId);
        node.setName(name);
        node.setType(FileNode.NodeType.FOLDER);
        fileNodeRepo.save(node);
        publishEvent(ownerId, node.getId(), "file.created", Map.of("name", name, "type", "FOLDER"));
        return node;
    }

    @Transactional
    public FileNode uploadFile(String ownerId, UUID parentId, MultipartFile file) throws IOException {
        byte[] bytes = file.getBytes();
        String checksum = hex(md5(bytes));
        String objectKey = ownerId + "/" + UUID.randomUUID() + "/" + file.getOriginalFilename();

        objectStore.put(objectKey, file.getInputStream(), file.getSize(), file.getContentType());

        FileNode node = fileNodeRepo
                .findByOwnerIdAndParentIdAndDeletedAtIsNull(ownerId, parentId)
                .stream()
                .filter(n -> n.getName().equals(file.getOriginalFilename()) && n.getType() == FileNode.NodeType.FILE)
                .findFirst()
                .orElseGet(() -> {
                    FileNode n = new FileNode();
                    n.setOwnerId(ownerId);
                    n.setParentId(parentId);
                    n.setName(file.getOriginalFilename());
                    n.setType(FileNode.NodeType.FILE);
                    return n;
                });

        node.setSizeBytes(file.getSize());
        node.setUpdatedAt(Instant.now());
        fileNodeRepo.save(node);

        int nextVersion = fileVersionRepo.findTopByFileIdOrderByVersionDesc(node.getId())
                .map(v -> v.getVersion() + 1).orElse(1);

        FileVersion version = new FileVersion();
        version.setFileId(node.getId());
        version.setVersion(nextVersion);
        version.setObjectKey(objectKey);
        version.setChecksum(checksum);
        version.setSizeBytes(file.getSize());
        fileVersionRepo.save(version);

        publishEvent(ownerId, node.getId(), "file.version_added",
                Map.of("version", nextVersion, "checksum", checksum));
        return node;
    }

    public List<FileNode> listFolder(String ownerId, UUID parentId) {
        return fileNodeRepo.findByOwnerIdAndParentIdAndDeletedAtIsNull(ownerId, parentId);
    }

    public FileNode getFile(String ownerId, UUID fileId) {
        return fileNodeRepo.findByIdAndOwnerIdAndDeletedAtIsNull(fileId, ownerId)
                .orElseThrow(() -> new NoSuchElementException("File not found: " + fileId));
    }

    @Transactional
    public void deleteFile(String ownerId, UUID fileId) {
        FileNode node = getFile(ownerId, fileId);
        node.setDeletedAt(Instant.now());
        fileNodeRepo.save(node);
        publishEvent(ownerId, fileId, "file.deleted", Map.of());
    }

    public List<ChangeEvent> syncFeed(String ownerId, long afterEventId) {
        return changeEventRepo.findByOwnerIdAndIdGreaterThanOrderByIdAsc(ownerId, afterEventId);
    }

    private void publishEvent(String ownerId, UUID fileId, String eventType, Map<String, Object> payload) {
        ChangeEvent ev = new ChangeEvent();
        ev.setOwnerId(ownerId);
        ev.setFileId(fileId);
        ev.setEventType(eventType);
        ev.setPayload(payload);
        changeEventRepo.save(ev);
        kafkaTemplate.send("dropbox.file-events", ownerId, Map.of(
                "eventType", eventType, "fileId", fileId.toString(), "payload", payload));
    }

    private byte[] md5(byte[] data) {
        try {
            return MessageDigest.getInstance("MD5").digest(data);
        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }

    private String hex(byte[] bytes) {
        StringBuilder sb = new StringBuilder();
        for (byte b : bytes) sb.append(String.format("%02x", b));
        return sb.toString();
    }
}
