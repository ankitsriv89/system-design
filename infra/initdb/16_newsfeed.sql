-- Provision database and user for project 16: news-feed-system
CREATE USER newsfeed WITH PASSWORD 'newsfeed';
CREATE DATABASE newsfeed OWNER newsfeed;
