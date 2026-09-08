-- The platform's commission is added on top of the ride price charged to
-- the passenger — the driver always receives driver_earning in full, she
-- never has the fee taken out of what she earned. `price` (already
-- existing) is the total charged to the passenger: driver_earning + platform_fee.
ALTER TABLE rides ADD COLUMN driver_earning REAL NOT NULL DEFAULT 0;
ALTER TABLE rides ADD COLUMN platform_fee REAL NOT NULL DEFAULT 0;
