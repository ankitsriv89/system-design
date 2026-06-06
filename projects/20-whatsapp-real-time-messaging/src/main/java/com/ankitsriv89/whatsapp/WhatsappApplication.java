package com.ankitsriv89.whatsapp;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.scheduling.annotation.EnableScheduling;

@SpringBootApplication
@EnableScheduling
public class WhatsappApplication {
    public static void main(String[] args) {
        SpringApplication.run(WhatsappApplication.class, args);
    }
}
