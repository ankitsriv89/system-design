-- 00_ratelimiter.sql
-- Creates the database and user for project 01 (Rate Limiter).
-- Runs automatically on postgres first start via docker-entrypoint-initdb.d.

CREATE USER rl WITH PASSWORD 'rl';
CREATE DATABASE ratelimiter OWNER rl;
GRANT ALL PRIVILEGES ON DATABASE ratelimiter TO rl;
