-- +goose Up
ALTER TABLE requests.request_items ADD COLUMN unit_label text NOT NULL DEFAULT 'шт.';
