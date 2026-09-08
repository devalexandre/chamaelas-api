-- What Pagar.me itself charges the platform per payment method — separate
-- from our own commission to drivers. Kept here so we know our real cost
-- and can calculate net margin per payment method later. Seeded with the
-- "Plano à Vista" rates as a starting point; edit freely in the panel.
CREATE TABLE gateway_fee_rates (
    id             TEXT PRIMARY KEY,
    payment_method TEXT NOT NULL UNIQUE,
    label          TEXT NOT NULL,
    fee_percent    REAL NOT NULL,
    sort_order     INTEGER NOT NULL DEFAULT 0
);

INSERT INTO gateway_fee_rates (id, payment_method, label, fee_percent, sort_order) VALUES
    ('pix', 'pix', 'Pix', 1.19, 1),
    ('credit_1x', 'credit_1x', 'Crédito 1x', 4.39, 2),
    ('credit_6x', 'credit_6x', 'Crédito 6x', 14.99, 3),
    ('credit_12x', 'credit_12x', 'Crédito 12x', 25.29, 4);
