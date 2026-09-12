-- Replaces the single-driver "offer with a timeout" model: a ride simply
-- stays "searching" and every eligible online driver's app can see it,
-- nearest first, until someone accepts. A driver who declines just stops
-- seeing that one ride again; everyone else still can. There's no timeout —
-- the offer only goes away by an explicit accept/decline, or another driver
-- taking the ride first.
ALTER TABLE rides DROP COLUMN offered_driver_id;
ALTER TABLE rides DROP COLUMN offer_expires_at;

CREATE TABLE ride_declines (
    ride_id    TEXT NOT NULL REFERENCES rides(id),
    driver_id  TEXT NOT NULL REFERENCES drivers(id),
    created_at TIMESTAMP NOT NULL,
    PRIMARY KEY (ride_id, driver_id)
);
