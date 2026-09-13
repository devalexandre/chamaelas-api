-- Nullable, globally unique (excluding NULLs) Google account identifier.
-- NULL means no Google account linked — she can still have a password.
ALTER TABLE users ADD COLUMN google_sub TEXT;
CREATE UNIQUE INDEX idx_users_google_sub ON users(google_sub) WHERE google_sub IS NOT NULL;
