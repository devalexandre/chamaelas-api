-- In-ride chat between passenger and driver — only usable while a ride is
-- assigned to a driver and still open (not completed/cancelled; enforced at
-- the handler, not here). Kept forever afterward as a read-only transcript.
CREATE TABLE ride_messages (
    id          TEXT PRIMARY KEY,
    ride_id     TEXT NOT NULL REFERENCES rides(id),
    sender_type TEXT NOT NULL, -- 'user' | 'driver'
    sender_id   TEXT NOT NULL,
    body        TEXT NOT NULL,
    created_at  TIMESTAMP NOT NULL
);

CREATE INDEX idx_ride_messages_ride ON ride_messages(ride_id, created_at);
