package com.ankitsriv89.dropbox.repository;

import com.ankitsriv89.dropbox.domain.ChangeEvent;
import org.springframework.data.jpa.repository.JpaRepository;
import java.util.List;

public interface ChangeEventRepository extends JpaRepository<ChangeEvent, Long> {
    List<ChangeEvent> findByOwnerIdAndIdGreaterThanOrderByIdAsc(String ownerId, long afterId);
}
