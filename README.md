# Frame

A film-taste dating app. No photos. Just cinema.

## Stack

| Layer | Tech |
|-------|------|
| Frontend | Next.js 15 + TypeScript |
| Backend API | Go + Fiber |
| ML Service | Python + FastAPI |
| Database | Postgres + pgvector |
| Queue | Redis + Asynq |
| Deploy | Railway |

## Getting Started

### Prerequisites
- Go 1.22+
- Node.js 20+
- Python 3.11+
- Docker + Docker Compose

### 1. Clone and configure

```bash
git clone https://github.com/your-username/frame
cd frame
cp .env.example .env
# edit .env and fill in your API keys
```

### 2. Start the database

```bash
make dev
# this starts postgres (with pgvector) and redis via docker compose
```

### 3. Run migrations

```bash
make db-migrate
```

### 4. Start the API

```bash
make dev-api
```

### 5. Start the frontend

```bash
cd apps/web
npm install
make dev-web
```

## Project Structure

```
frame/
├── apps/
│   └── web/                  # Next.js frontend
├── services/
│   ├── api/                  # Go backend
│   │   ├── cmd/server/       # Entry point
│   │   ├── internal/         # Business logic (unexported)
│   │   │   ├── scraper/      # Letterboxd scraping
│   │   │   ├── enrichment/   # TMDB enrichment
│   │   │   ├── vector/       # Taste vector computation
│   │   │   ├── matching/     # pgvector matching queries
│   │   │   ├── llm/          # Claude API calls
│   │   │   └── jobs/         # Background job workers
│   │   ├── pkg/              # Shared utilities (exported)
│   │   │   ├── config/       # Env config
│   │   │   ├── db/           # DB connection pool
│   │   │   └── models/       # Domain structs
│   │   └── migrations/       # SQL migrations
│   └── ml/                   # Python embedding service
├── packages/
│   └── types/                # Shared TypeScript types
└── infra/
    └── docker-compose.yml    # Local dev infrastructure
```

## Key Commands

```bash
make dev          # start postgres + redis
make dev-api      # start Go API (hot reload)
make dev-web      # start Next.js frontend
make db-migrate   # run pending migrations
make test-api     # run Go tests
make tidy         # install all dependencies
```
