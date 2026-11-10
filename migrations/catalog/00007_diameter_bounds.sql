-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';
ALTER TABLE catalog.tyre_specs ADD CONSTRAINT tyre_specs_diameter_range
 CHECK (diameter IS NULL OR diameter BETWEEN 11 AND 25);
ALTER TABLE catalog.wheel_specs ADD CONSTRAINT wheel_specs_diameter_range
 CHECK (diameter IS NULL OR diameter BETWEEN 11 AND 25);
