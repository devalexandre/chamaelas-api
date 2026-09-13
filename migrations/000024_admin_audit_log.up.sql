-- Every admin action that touches money or a payout destination gets
-- logged here — who did it, what changed, when — so it's easy to audit
-- after the fact instead of trusting an unlogged form submission.
-- admin_name is denormalized (kept even if the admin account is later
-- removed) specifically so the log stays readable on its own.
CREATE TABLE admin_audit_log (
    id          TEXT PRIMARY KEY,
    admin_id    TEXT NOT NULL,
    admin_name  TEXT NOT NULL,
    action      TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id   TEXT NOT NULL,
    details     TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMP NOT NULL
);

CREATE INDEX idx_admin_audit_log_target ON admin_audit_log(target_type, target_id);
