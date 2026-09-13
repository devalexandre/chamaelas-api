-- Audit-only ledger for a driver's revenue wallet (her Woovi subaccount
-- balance, credited from ride.DriverEarning on each completed ride). The
-- spendable balance itself is never stored here — always read live from
-- Woovi — this table exists so a skipped/failed credit (no Pix key yet, a
-- transfer error) is a visible, reconcilable row instead of a silent gap.
CREATE TABLE wallet_transactions (
    id                      TEXT PRIMARY KEY,
    driver_id               TEXT NOT NULL REFERENCES drivers(id),
    type                    TEXT NOT NULL, -- 'ride_earning' | 'withdrawal' | 'withdrawal_fee'
    amount                  REAL NOT NULL,
    ride_id                 TEXT REFERENCES rides(id),
    provider_transaction_id TEXT,
    note                    TEXT NOT NULL DEFAULT '',
    created_at              TIMESTAMP NOT NULL
);

CREATE INDEX idx_wallet_transactions_driver ON wallet_transactions(driver_id);

-- Tracks a driver's prepaid-credit top-up charges (PIX, no split — 100%
-- stays with the platform). id = Woovi's correlationID, so the webhook can
-- look up "which driver, how much" from a bare correlationID alone.
CREATE TABLE credit_topups (
    id           TEXT PRIMARY KEY,
    driver_id    TEXT NOT NULL REFERENCES drivers(id),
    amount_cents INTEGER NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending', -- 'pending' | 'paid' | 'expired'
    created_at   TIMESTAMP NOT NULL,
    paid_at      TIMESTAMP
);

-- Webhook delivery dedup: Woovi (and any future provider) can redeliver the
-- same event, and the same charge legitimately produces more than one event
-- type over its life (paid, then later expired) — event_key includes the
-- event type, not just the correlation ID, so those aren't confused.
CREATE TABLE webhook_events (
    id         TEXT PRIMARY KEY,
    provider   TEXT NOT NULL,
    event_key  TEXT NOT NULL,
    payload    TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL
);

CREATE UNIQUE INDEX idx_webhook_events_dedup ON webhook_events(provider, event_key);
