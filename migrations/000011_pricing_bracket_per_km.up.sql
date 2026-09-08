-- A bracket's final price is base price + (price_per_km * distance) — the
-- existing "price" column becomes the fixed component, defaulting
-- price_per_km to 0 keeps flat-price brackets working unchanged.
ALTER TABLE pricing_brackets ADD COLUMN price_per_km REAL NOT NULL DEFAULT 0;
