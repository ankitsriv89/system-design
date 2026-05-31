-- 01_urlshortener.sql
-- Creates the database and user for project 02 (URL Shortener).
-- This script runs automatically on postgres first start via docker-entrypoint-initdb.d.
-- Add similar scripts for future projects (02_nextproject.sql, etc.).

CREATE USER url WITH PASSWORD 'url';
CREATE DATABASE urlshortener OWNER url;
GRANT ALL PRIVILEGES ON DATABASE urlshortener TO url;
