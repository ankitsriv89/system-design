package com.ankitsriv89.proximityservice;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.scheduling.annotation.EnableScheduling;

@SpringBootApplication
@EnableScheduling
public class ProximityServiceApplication {
    public static void main(String[] args) {
        SpringApplication.run(ProximityServiceApplication.class, args);
    }
}
