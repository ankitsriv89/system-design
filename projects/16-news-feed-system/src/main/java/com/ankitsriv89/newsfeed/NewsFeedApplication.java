package com.ankitsriv89.newsfeed;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.scheduling.annotation.EnableScheduling;

@SpringBootApplication
@EnableScheduling
public class NewsFeedApplication {
    public static void main(String[] args) {
        SpringApplication.run(NewsFeedApplication.class, args);
    }
}
