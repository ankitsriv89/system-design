package com.ankitsriv89.dropbox.repository;

import com.ankitsriv89.dropbox.domain.FileVersion;
import org.springframework.data.jpa.repository.JpaRepository;
import java.util.List;
import java.util.Optional;
import java.util.UUID;

public interface FileVersionRepository extends JpaRepository<FileVersion, Long> {
    List<FileVersion> findByFileIdOrderByVersionDesc(UUID fileId);
    Optional<FileVersion> findTopByFileIdOrderByVersionDesc(UUID fileId);
}
