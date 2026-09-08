-- final_price is set when the ride actually completes (NULL for a
-- cancelled/in-progress ride, since it never reached a final value).
-- amount_paid is set once real payment capture exists — NULL until then,
-- so the report shows "—" honestly instead of a made-up number.
ALTER TABLE rides ADD COLUMN final_price REAL;
ALTER TABLE rides ADD COLUMN amount_paid REAL;
