-- Pagar.me gateway credentials and the platform's own recipient (our
-- "carteira"/wallet, separate from each driver's individual recebedor).
-- Single row, edited from the admin panel's financial section.
CREATE TABLE payment_settings (
    id                    INTEGER PRIMARY KEY,
    environment           TEXT NOT NULL DEFAULT 'sandbox', -- 'sandbox' | 'production'
    public_key            TEXT NOT NULL DEFAULT '',
    secret_key            TEXT NOT NULL DEFAULT '',
    platform_recipient_id TEXT NOT NULL DEFAULT ''
);

INSERT INTO payment_settings (id, environment) VALUES (1, 'sandbox');
