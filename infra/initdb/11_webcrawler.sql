-- Provision database and user for project 12 — web-crawler
CREATE USER crawler WITH PASSWORD 'crawler';
CREATE DATABASE crawler OWNER crawler;
