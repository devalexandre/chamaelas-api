CREATE TABLE rides (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id),
    driver_id    TEXT REFERENCES drivers(id),
    origin       TEXT NOT NULL,
    destination  TEXT NOT NULL,
    distance_km  REAL NOT NULL,
    duration_min INTEGER NOT NULL,
    price        REAL NOT NULL,
    status       TEXT NOT NULL,
    rating       INTEGER,
    created_at   TIMESTAMP NOT NULL
);

CREATE INDEX idx_rides_user_id ON rides(user_id);
