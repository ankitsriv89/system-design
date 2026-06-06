-- Project 21: Dropbox File Storage and Sync
CREATE USER dropbox WITH PASSWORD 'dropbox';
CREATE DATABASE dropbox OWNER dropbox;
GRANT ALL PRIVILEGES ON DATABASE dropbox TO dropbox;
