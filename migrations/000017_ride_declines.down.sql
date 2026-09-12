DROP TABLE ride_declines;

ALTER TABLE rides ADD COLUMN offered_driver_id TEXT REFERENCES drivers(id);
ALTER TABLE rides ADD COLUMN offer_expires_at TIMESTAMP;
