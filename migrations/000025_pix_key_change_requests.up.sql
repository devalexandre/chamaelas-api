-- A driver's or passenger's request to change her Pix key doesn't take
-- effect immediately — an admin must approve it first, since a compromised
-- account could otherwise redirect a driver's real payout to a key that
-- isn't hers. Re-entering her password to submit the request (enforced at
-- the handler, not here) proves it's really her asking; the admin approval
-- is the second, independent check before any money can move differently.
CREATE TABLE pix_key_change_requests (
    id           TEXT PRIMARY KEY,
    owner_type   TEXT NOT NULL, -- 'driver' | 'user'
    owner_id     TEXT NOT NULL,
    old_pix_key  TEXT NOT NULL DEFAULT '',
    new_pix_key  TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending', -- 'pending' | 'approved' | 'rejected'
    requested_at TIMESTAMP NOT NULL,
    reviewed_at  TIMESTAMP,
    reviewed_by  TEXT,
    note         TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_pix_key_change_requests_owner ON pix_key_change_requests(owner_type, owner_id);
CREATE INDEX idx_pix_key_change_requests_status ON pix_key_change_requests(status);
