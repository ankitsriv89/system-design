-- Provision user and database for project 07-api-gateway.
-- Runs automatically on first PostgreSQL container start via docker-entrypoint-initdb.d.

CREATE USER gw WITH PASSWORD 'gw';
CREATE DATABASE apigateway OWNER gw;
GRANT ALL PRIVILEGES ON DATABASE apigateway TO gw;
