-- NULL means "use platform_settings.commission_rate" (the global default);
-- a non-NULL value overrides it for rides whose origin matches this city.
ALTER TABLE cities ADD COLUMN commission_rate REAL;
