ALTER TABLE drivers ADD COLUMN google_sub TEXT;
CREATE UNIQUE INDEX idx_drivers_google_sub ON drivers(google_sub) WHERE google_sub IS NOT NULL;
