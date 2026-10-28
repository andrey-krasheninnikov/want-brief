.PHONY: up down api api-dev db-dev web web-dev test lint migrate migrate-dev tidy

export GOTOOLCHAIN := local

DATABASE_URL ?= postgres://wantbrief:wantbrief@127.0.0.1:5432/wantbrief?sslmode=disable
DEV_DATABASE_URL ?= postgres://wantbrief:wantbrief@127.0.0.1:5432/wantbrief_dev?sslmode=disable
HTTP_ADDR ?= :8080
DEV_HTTP_ADDR ?= :8081
CORS_ORIGIN ?= http://127.0.0.1:5173
DEV_CORS_ORIGIN ?= http://127.0.0.1:5174
BOOTSTRAP_PASSWORD ?= wantbrief
TOKEN_KEY ?= 00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff

up:
	docker compose up -d
	@until docker compose exec -T postgres pg_isready -U wantbrief -d wantbrief >/dev/null 2>&1; do sleep 1; done

down:
	docker compose down

tidy:
	go mod tidy

migrate: up
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/api -migrate-only

db-dev: up
	-docker compose exec -T postgres createdb -U wantbrief wantbrief_dev

migrate-dev: db-dev
	DATABASE_URL="$(DEV_DATABASE_URL)" go run ./cmd/api -migrate-only

api: up
	DATABASE_URL="$(DATABASE_URL)" HTTP_ADDR="$(HTTP_ADDR)" CORS_ORIGIN="$(CORS_ORIGIN)" \
		BOOTSTRAP_PASSWORD="$(BOOTSTRAP_PASSWORD)" TOKEN_KEY="$(TOKEN_KEY)" \
		go run ./cmd/api

api-dev: db-dev
	DATABASE_URL="$(DEV_DATABASE_URL)" HTTP_ADDR="$(DEV_HTTP_ADDR)" CORS_ORIGIN="$(DEV_CORS_ORIGIN)" \
		BOOTSTRAP_PASSWORD="$(BOOTSTRAP_PASSWORD)" TOKEN_KEY="$(TOKEN_KEY)" \
		go run ./cmd/api

web:
	cd web && WEB_PORT=5173 API_PROXY=http://127.0.0.1:8080 bun run dev

web-dev:
	cd web && WEB_PORT=5174 API_PROXY=http://127.0.0.1:8081 bun run dev

test:
	go test ./...
	cd web && bun run typecheck

lint:
	gofmt -w $$(find . -name '*.go' -not -path './web/*')
	go vet ./...
	cd web && bun run typecheck

typecheck:
	cd web && bun run typecheck
