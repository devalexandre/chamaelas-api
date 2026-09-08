# chamaelas-api

Backend da passageira do Chama Elas. Echo + [ksql](https://github.com/VinGarcia/ksql) +
[golang-migrate](https://github.com/golang-migrate/migrate). SQLite por padrão (zero setup),
troca para Postgres só mudando duas variáveis de ambiente.

## Rodando em desenvolvimento (hot reload)

```bash
air
```

Isso builda, aplica as migrations e sobe o servidor em `http://localhost:8080`, recarregando a
cada alteração em `.go`, `.sql` ou `.env`. Sem `air`, o equivalente manual é `go run .`.

## Trocando de banco

Copie `.env.example` para `.env` e ajuste:

```bash
# SQLite (padrão, dev local)
DB_DRIVER=sqlite
DB_DSN=./data/chamaelas.db

# Postgres (QA/produção)
DB_DRIVER=postgres
DB_DSN=postgres://user:password@localhost:5432/chamaelas?sslmode=disable
```

Nenhum outro arquivo precisa mudar — `internal/database/database.go` escolhe o adapter do ksql
(`ksqlite` ou `kpgx`) e a migration runner (`golang-migrate`) a partir de `DB_DRIVER`. As mesmas
migrations em `migrations/*.sql` rodam nos dois bancos.

## Endpoints

| Método | Rota                     | Descrição                                   |
|--------|--------------------------|----------------------------------------------|
| GET    | `/healthz`               | Status + driver de banco em uso              |
| POST   | `/api/auth/signup`       | Cria usuária                                  |
| POST   | `/api/auth/login`        | Login                                         |
| GET    | `/api/drivers`           | Lista motoristas (dados seedados)             |
| POST   | `/api/rides`             | Solicita corrida                              |
| GET    | `/api/rides/:id`         | Status/detalhe da corrida                     |
| GET    | `/api/rides?userId=...`  | Histórico da usuária                          |
| POST   | `/api/rides/:id/cancel`  | Cancela (só antes da motorista chegar)        |
| POST   | `/api/rides/:id/rating`  | Avalia a corrida concluída                    |

Não existe ainda um app da motorista de verdade, então a própria API simula a progressão da
corrida (`buscando → aceita → chegou → em andamento → concluída`) num goroutine com os mesmos
tempos que o front usava no mock local — o app da passageira só precisa dar polling em
`GET /api/rides/:id`.

## Apontando o app da passageira para a API

No `chamaelas-usuario/frontend`, aponte as chamadas para `http://localhost:8080/api` (crie um
`services/api.ts` lá que substitua os mocks de `authService`/`rideService` por `fetch` reais).
