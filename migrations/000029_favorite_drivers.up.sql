-- A passenger's favorite drivers. During a short grace window right after
-- a ride is requested, GetOffer only offers it to a passenger's favorites
-- (if she has any) — everyone else becomes eligible once that window
-- passes, so a favorite driver isn't required to be online for the ride to
-- go out at all.
CREATE TABLE favorite_drivers (
    user_id    TEXT NOT NULL REFERENCES users(id),
    driver_id  TEXT NOT NULL REFERENCES drivers(id),
    created_at TIMESTAMP NOT NULL,
    PRIMARY KEY (user_id, driver_id)
);

CREATE INDEX idx_favorite_drivers_user ON favorite_drivers(user_id);
