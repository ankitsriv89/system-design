package com.ankitsriv89.instagram.service;

import com.ankitsriv89.instagram.domain.User;
import com.ankitsriv89.instagram.repository.UserRepository;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.CommandLineRunner;
import org.springframework.stereotype.Component;

import java.util.List;

/**
 * Seeds a handful of demo users on first boot so the UI has accounts to act as
 * (the project uses an X-User-Id header rather than full auth).
 */
@Component
public class DataSeeder implements CommandLineRunner {

    private static final Logger log = LoggerFactory.getLogger(DataSeeder.class);
    private static final List<String> DEMO_USERS = List.of("alice", "bob", "carol", "dave");

    private final UserRepository userRepository;

    public DataSeeder(UserRepository userRepository) {
        this.userRepository = userRepository;
    }

    @Override
    public void run(String... args) {
        for (String username : DEMO_USERS) {
            userRepository.findByUsername(username)
                    .orElseGet(() -> userRepository.save(new User(username)));
        }
        log.info("Seeded demo users: {}", DEMO_USERS);
    }
}
