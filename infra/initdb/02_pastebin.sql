-- 02_pastebin.sql
-- Creates the database and user for project 03 (Pastebin).
-- Runs automatically on postgres first start via docker-entrypoint-initdb.d.

CREATE USER paste WITH PASSWORD 'paste';
CREATE DATABASE pastebin OWNER paste;
GRANT ALL PRIVILEGES ON DATABASE pastebin TO paste;
