CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    email         TEXT NOT NULL UNIQUE,
    phone         TEXT NOT NULL DEFAULT '',
    cpf           TEXT NOT NULL DEFAULT '',
    birth_date    TEXT NOT NULL DEFAULT '',
    photo_url     TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMP NOT NULL
);
