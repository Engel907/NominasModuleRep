# Solo para desarrollo / CI. Las migraciones en otros ambientes se ejecutan
# de forma deliberada con: go run ./cmd/migrate up
.PHONY: db-up migrate-up migrate-down migrate-version db-reset

N ?= 1
MIGRATE := go run ./cmd/migrate

db-up:
	docker compose up -d --wait mysql

migrate-up:
	$(MIGRATE) up

# Revierte N migraciones (por defecto 1): make migrate-down N=2
migrate-down:
	$(MIGRATE) down $(N)

migrate-version:
	$(MIGRATE) version

# Destruye el volumen de MySQL local y recrea todo. Solo desarrollo.
# Se niega si APP_ENV (del entorno del shell) es production o staging.
db-reset:
	@if [ "$(APP_ENV)" = "production" ] || [ "$(APP_ENV)" = "staging" ]; then echo "db-reset es solo para desarrollo (APP_ENV=$(APP_ENV))"; exit 1; fi
	docker compose down -v
	docker compose up -d --wait mysql
	$(MIGRATE) up