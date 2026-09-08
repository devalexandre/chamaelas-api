-- Two commission collection modes, chosen by the driver at signup and
-- changeable later from her profile:
--   per_ride: the platform's fee is split out of each ride's payment
--             at the gateway (Pagar.me) when a real charge happens.
--   prepaid:  the driver preloads credit; the platform's fee is debited
--             from that balance whenever a ride completes.
ALTER TABLE drivers ADD COLUMN billing_mode TEXT NOT NULL DEFAULT 'per_ride';
ALTER TABLE drivers ADD COLUMN credit_balance REAL NOT NULL DEFAULT 0;

CREATE TABLE credit_transactions (
    id          TEXT PRIMARY KEY,
    driver_id   TEXT NOT NULL REFERENCES drivers(id),
    type        TEXT NOT NULL, -- 'topup' | 'ride_fee' | 'adjustment'
    amount      REAL NOT NULL, -- positive for topup/adjustment credit, negative for debits
    ride_id     TEXT REFERENCES rides(id),
    balance_after REAL NOT NULL,
    note        TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMP NOT NULL
);

CREATE INDEX idx_credit_transactions_driver ON credit_transactions(driver_id);

-- Single-row table holding platform-wide financial settings, edited from
-- the admin panel's financial section.
CREATE TABLE platform_settings (
    id              INTEGER PRIMARY KEY,
    commission_rate REAL NOT NULL DEFAULT 0.20
);

INSERT INTO platform_settings (id, commission_rate) VALUES (1, 0.20);
