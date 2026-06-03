-- Provision database and user for project 10: notification-system
CREATE USER notif WITH PASSWORD 'notif';
CREATE DATABASE notifications OWNER notif;
