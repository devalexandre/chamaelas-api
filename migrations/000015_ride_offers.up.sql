-- offered_driver_id/offer_expires_at track a pending offer to one specific
-- driver — the ride stays in "searching" status (the passenger sees no
-- difference) until she accepts; a decline or timeout clears these and the
-- matcher offers the next-nearest driver instead of auto-assigning.
ALTER TABLE rides ADD COLUMN offered_driver_id TEXT REFERENCES drivers(id);
ALTER TABLE rides ADD COLUMN offer_expires_at TIMESTAMP;
