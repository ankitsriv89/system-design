-- 03_uniqueid.sql
-- Creates the database and user for project 04 (Unique ID Generator).
-- Runs automatically on postgres first start via docker-entrypoint-initdb.d.

CREATE USER uniqueid WITH PASSWORD 'uniqueid';
CREATE DATABASE uniqueid OWNER uniqueid;
GRANT ALL PRIVILEGES ON DATABASE uniqueid TO uniqueid;
