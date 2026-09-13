-- Auto-created on login (or admin approval, for drivers) using the CPF as
-- the default Pix key if she/he hasn't got one yet — see PixKey doc on
-- models.Driver/models.User. Same canonical-key convention as drivers.
ALTER TABLE users ADD COLUMN pix_key TEXT NOT NULL DEFAULT '';
