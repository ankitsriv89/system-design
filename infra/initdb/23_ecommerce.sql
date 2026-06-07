-- Project 23: E-Commerce Platform
-- Creates the ecommerce user and database.
-- Schema migrations are applied by Flyway on first boot (V1__init.sql).

CREATE USER ecommerce WITH PASSWORD 'ecommerce';
CREATE DATABASE ecommerce OWNER ecommerce;
GRANT ALL PRIVILEGES ON DATABASE ecommerce TO ecommerce;
