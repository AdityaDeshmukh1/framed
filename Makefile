.PHONY: dev build test clean

# ── local dev ──────────────────────────────────────────────────────────────────
dev:
	docker compose -f infra/docker-compose.yml up

dev-api:
	cd services/api && go run cmd/server/main.go

dev-web:
	cd apps/web && npm run dev

dev-ml:
	cd services/ml && uvicorn main:app --reload --port 8001

# ── build ──────────────────────────────────────────────────────────────────────
build-api:
	cd services/api && go build -o bin/api cmd/server/main.go

build-web:
	cd apps/web && npm run build

# ── database ───────────────────────────────────────────────────────────────────
db-migrate:
	cd services/api && go run cmd/migrate/main.go up

db-rollback:
	cd services/api && go run cmd/migrate/main.go down

db-reset:
	cd services/api && go run cmd/migrate/main.go reset

# ── test ───────────────────────────────────────────────────────────────────────
test-api:
	cd services/api && go test ./...

test-web:
	cd apps/web && npm run test

# ── clean ──────────────────────────────────────────────────────────────────────
clean:
	cd services/api && rm -rf bin/
	cd apps/web && rm -rf .next/

# ── utils ──────────────────────────────────────────────────────────────────────
tidy:
	cd services/api && go mod tidy
	cd services/ml && pip install -r requirements.txt
	cd apps/web && npm install
