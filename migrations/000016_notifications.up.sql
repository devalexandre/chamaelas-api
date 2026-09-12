-- In-app notifications the admin panel sends to drivers and/or passengers.
-- No push infrastructure exists yet, so delivery is by polling: each app
-- checks its own list of unread rows for its user/driver id.
CREATE TABLE notifications (
    id             TEXT PRIMARY KEY,
    recipient_type TEXT NOT NULL, -- 'driver' or 'user'
    recipient_id   TEXT NOT NULL,
    title          TEXT NOT NULL,
    message        TEXT NOT NULL,
    created_at     TIMESTAMP NOT NULL,
    read_at        TIMESTAMP
);

CREATE INDEX idx_notifications_recipient ON notifications(recipient_type, recipient_id);
