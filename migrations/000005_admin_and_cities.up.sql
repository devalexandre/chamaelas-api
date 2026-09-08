CREATE TABLE admins (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMP NOT NULL
);

CREATE TABLE cities (
    id     TEXT PRIMARY KEY,
    name   TEXT NOT NULL,
    uf     TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE
);

INSERT INTO cities (id, name, uf, active) VALUES
    ('sao-paulo-sp', 'São Paulo', 'SP', TRUE);
