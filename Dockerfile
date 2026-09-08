# Migrations are embedded (see main.go's //go:embed) and both DB adapters
# (modernc sqlite, pgx for Postgres) are pure Go, so this needs no CGO and
# no files copied alongside the binary at runtime.
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /chamaelas-api .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /chamaelas-api /chamaelas-api

# Render (and most PaaS) inject PORT themselves; the app already reads it.
EXPOSE 8080
ENTRYPOINT ["/chamaelas-api"]
