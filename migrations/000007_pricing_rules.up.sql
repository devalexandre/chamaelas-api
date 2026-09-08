-- A pricing rule is a day-of-week + time window (e.g. "segunda 08:00-10:00"),
-- optionally scoped to one city and/or category (NULL = applies to all).
-- Each rule has one or more km brackets with their own price, so a single
-- rule can express "0-5km = R$25, 5.1-20km = R$30" in that window.
CREATE TABLE pricing_rules (
    id            TEXT PRIMARY KEY,
    city_id       TEXT REFERENCES cities(id),
    category_id   TEXT REFERENCES categories(id),
    day_of_week   INTEGER NOT NULL, -- 0 = domingo .. 6 = sabado
    start_time    TEXT NOT NULL,    -- "HH:MM"
    end_time      TEXT NOT NULL,    -- "HH:MM"
    created_at    TIMESTAMP NOT NULL
);

CREATE TABLE pricing_brackets (
    id        TEXT PRIMARY KEY,
    rule_id   TEXT NOT NULL REFERENCES pricing_rules(id),
    km_from   REAL NOT NULL,
    km_to     REAL, -- NULL = sem limite superior
    price     REAL NOT NULL
);

CREATE INDEX idx_pricing_rules_day ON pricing_rules(day_of_week);
CREATE INDEX idx_pricing_brackets_rule ON pricing_brackets(rule_id);
