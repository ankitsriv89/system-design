package com.ankitsriv89.groupchat.repository;

import com.ankitsriv89.groupchat.domain.Group;
import org.springframework.data.jpa.repository.JpaRepository;

public interface GroupRepository extends JpaRepository<Group, Long> {}
