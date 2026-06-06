package com.ankitsriv89.dropbox.repository;

import com.ankitsriv89.dropbox.domain.SyncCursor;
import org.springframework.data.jpa.repository.JpaRepository;

public interface SyncCursorRepository extends JpaRepository<SyncCursor, String> {
}
