-- A short free-text note the passenger can leave for the driver when
-- requesting a ride (building access code, "toque a campainha", pet in the
-- car, etc.) — optional, empty for most rides.
ALTER TABLE rides ADD COLUMN note TEXT;
