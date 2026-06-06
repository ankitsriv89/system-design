package com.ankitsriv89.dropbox.repository;

import com.ankitsriv89.dropbox.domain.FileNode;
import org.springframework.data.jpa.repository.JpaRepository;
import java.util.List;
import java.util.Optional;
import java.util.UUID;

public interface FileNodeRepository extends JpaRepository<FileNode, UUID> {
    List<FileNode> findByOwnerIdAndParentIdAndDeletedAtIsNull(String ownerId, UUID parentId);
    Optional<FileNode> findByIdAndOwnerIdAndDeletedAtIsNull(UUID id, String ownerId);
}
