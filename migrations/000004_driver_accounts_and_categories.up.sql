-- Turns "drivers" from mocked profile data into real driver accounts, and
-- adds ride categories. Dev-only data so far, so it's simplest to rebuild
-- rides/drivers from scratch rather than juggle per-engine ALTER TABLE syntax.
DROP TABLE rides;
DROP TABLE drivers;

CREATE TABLE drivers (
    id                  TEXT PRIMARY KEY,
    name                TEXT NOT NULL,
    email               TEXT NOT NULL UNIQUE,
    phone               TEXT NOT NULL DEFAULT '',
    cpf                 TEXT NOT NULL DEFAULT '',
    cnh                 TEXT NOT NULL DEFAULT '',
    birth_date          TEXT NOT NULL DEFAULT '',
    photo_url           TEXT NOT NULL DEFAULT '',
    password_hash       TEXT NOT NULL,
    vehicle_plate       TEXT NOT NULL DEFAULT '',
    vehicle_model       TEXT NOT NULL DEFAULT '',
    vehicle_color       TEXT NOT NULL DEFAULT '',
    vehicle_year        TEXT NOT NULL DEFAULT '',
    rating              REAL NOT NULL DEFAULT 5.0,
    -- Defaults to 'approved' until the admin panel exists to review new
    -- signups; switch this default to 'pending' once that panel ships.
    status              TEXT NOT NULL DEFAULT 'approved',
    is_online           BOOLEAN NOT NULL DEFAULT FALSE,
    lat                 REAL,
    lng                 REAL,
    location_updated_at TIMESTAMP,
    created_at          TIMESTAMP NOT NULL
);

CREATE TABLE categories (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    active      BOOLEAN NOT NULL DEFAULT TRUE
);

INSERT INTO categories (id, name, description, active) VALUES
    ('standard', 'Padrão', 'Carro popular, ideal para o dia a dia', TRUE),
    ('comfort', 'Conforto', 'Carro maior e mais novo', TRUE),
    ('pet', 'Pet Friendly', 'Motorista aceita levar pets', TRUE);

CREATE TABLE driver_categories (
    driver_id   TEXT NOT NULL REFERENCES drivers(id),
    category_id TEXT NOT NULL REFERENCES categories(id),
    PRIMARY KEY (driver_id, category_id)
);

CREATE TABLE rides (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id),
    driver_id    TEXT REFERENCES drivers(id),
    category_id  TEXT NOT NULL DEFAULT 'standard' REFERENCES categories(id),
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
CREATE INDEX idx_drivers_online ON drivers(is_online, status);
