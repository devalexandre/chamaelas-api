-- Required to create a real Pagar.me Recebedor (recipient) for the driver
-- on approval — Pagar.me's v5 API requires a full bank account, a bare Pix
-- key alone isn't accepted by the documented Recipients endpoint.
ALTER TABLE drivers ADD COLUMN bank_code TEXT NOT NULL DEFAULT '';
ALTER TABLE drivers ADD COLUMN bank_branch TEXT NOT NULL DEFAULT '';
ALTER TABLE drivers ADD COLUMN bank_account_number TEXT NOT NULL DEFAULT '';
ALTER TABLE drivers ADD COLUMN bank_account_type TEXT NOT NULL DEFAULT 'checking'; -- checking | savings
ALTER TABLE drivers ADD COLUMN pagarme_recipient_id TEXT NOT NULL DEFAULT '';
