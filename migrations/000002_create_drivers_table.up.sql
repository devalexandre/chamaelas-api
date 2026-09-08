CREATE TABLE drivers (
    id      TEXT PRIMARY KEY,
    name    TEXT NOT NULL,
    rating  REAL NOT NULL,
    car     TEXT NOT NULL,
    plate   TEXT NOT NULL,
    eta_min INTEGER NOT NULL
);

INSERT INTO drivers (id, name, rating, car, plate, eta_min) VALUES
    ('d1', 'Camila Souza',   4.9, 'Chevrolet Onix Branco', 'ABC1D23', 3),
    ('d2', 'Beatriz Lima',   4.8, 'Fiat Argo Prata',       'DEF4E56', 5),
    ('d3', 'Larissa Alves',  5.0, 'Hyundai HB20 Rosa',     'GHI7F89', 4),
    ('d4', 'Fernanda Costa', 4.7, 'Volkswagen Polo Preto', 'JKL0G12', 6),
    ('d5', 'Juliana Rocha',  4.9, 'Renault Kwid Branco',   'MNO3H45', 2);
