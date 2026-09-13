-- The driver's own Pix key, used both to identify her Woovi subaccount (her
-- revenue wallet, separate from her prepaid credit_balance) and as the
-- destination for a real payout when she withdraws. Always stored as the
-- CANONICAL key Woovi returns from EnsureRecipient, never the raw input the
-- driver typed — see internal/woovi.
ALTER TABLE drivers ADD COLUMN pix_key TEXT NOT NULL DEFAULT '';

-- Woovi (PIX gateway) credentials and platform-wide settings, single row,
-- edited from the admin panel's financial section — same shape as
-- payment_settings (Pagar.me).
CREATE TABLE woovi_settings (
    id                     INTEGER PRIMARY KEY,
    environment            TEXT NOT NULL DEFAULT 'sandbox', -- 'sandbox' | 'production'
    app_id                 TEXT NOT NULL DEFAULT '',
    webhook_secret         TEXT NOT NULL DEFAULT '',
    webhook_public_key_b64 TEXT NOT NULL DEFAULT '',
    platform_pix_key       TEXT NOT NULL DEFAULT ''
);

INSERT INTO woovi_settings (id, environment) VALUES (1, 'sandbox');
