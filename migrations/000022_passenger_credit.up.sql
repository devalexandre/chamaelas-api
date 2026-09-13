-- Mirrors drivers.credit_balance / credit_transactions, but for passengers:
-- a spend-only prepaid balance (never withdrawable) meant to pay for ride
-- prices. Kept as its own table rather than a nullable driver_id/user_id on
-- the existing ledger, since nothing else here shares one ledger table
-- across two different owner types.
ALTER TABLE users ADD COLUMN credit_balance REAL NOT NULL DEFAULT 0;

CREATE TABLE user_credit_transactions (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL REFERENCES users(id),
    type          TEXT NOT NULL, -- 'topup' | 'ride_payment' | 'adjustment'
    amount        REAL NOT NULL, -- positive for topup/adjustment credit, negative for debits
    ride_id       TEXT REFERENCES rides(id),
    balance_after REAL NOT NULL,
    note          TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMP NOT NULL
);

CREATE INDEX idx_user_credit_transactions_user ON user_credit_transactions(user_id);

-- Tracks a passenger's prepaid-credit top-up charges, same shape as
-- credit_topups but keyed to a user instead of a driver.
CREATE TABLE user_credit_topups (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id),
    amount_cents INTEGER NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending', -- 'pending' | 'paid' | 'expired'
    created_at   TIMESTAMP NOT NULL,
    paid_at      TIMESTAMP
);
