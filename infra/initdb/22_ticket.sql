-- Project 22: Ticket Booking System
-- Creates the ticketbooking user and database.
-- Schema migrations are applied by Flyway on first boot (V1__init.sql).

CREATE USER ticketbooking WITH PASSWORD 'ticketbooking';
CREATE DATABASE ticketbooking OWNER ticketbooking;
GRANT ALL PRIVILEGES ON DATABASE ticketbooking TO ticketbooking;
