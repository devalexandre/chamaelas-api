-- Populated once real charges go through Pagar.me — until then it stays
-- NULL and the "por meio de pagamento" report shows everything as
-- "Não informado" instead of fabricating a breakdown.
ALTER TABLE rides ADD COLUMN payment_method TEXT;
