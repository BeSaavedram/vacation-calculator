export PATH := $(PATH):/Applications/Docker.app/Contents/Resources/bin
include .env
export

.PHONY: db-up db-down migrate seed api web test demo-reset

db-up:
	docker compose up -d postgres;
	@echo "esperando a postgres..."
	@until docker compose exec -T postgres pg_isready -U vacaciones -d vacaciones >/dev/null 2>&1; do sleep 1; done
	@echo "postgres listo en localhost:5433"

db-down:
	docker compose down -v;

migrate:
	go run ./cmd/migrate

seed:
	go run ./cmd/seed

api:
	go run ./cmd/api

web:
	cd web && npm run dev

test:
	go test ./internal/... -v

demo-reset: db-down db-up migrate seed
