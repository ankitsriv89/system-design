-- 14_chat.sql — Database and user for the one-to-one chat system (project 14).
-- Runs automatically when Postgres first starts via docker-entrypoint-initdb.d.

CREATE USER chat WITH PASSWORD 'chat';
CREATE DATABASE chat OWNER chat;
GRANT ALL PRIVILEGES ON DATABASE chat TO chat;
