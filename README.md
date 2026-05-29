# Framed

A film-taste dating app. No photos. Just cinema.

Connect your Letterboxd. We scrape your ratings, build a taste fingerprint, and find people who see films the same way you do — or interestingly differently.

## Stack

| Layer | Tech |
|---|---|
| Frontend | Next.js 15 + TypeScript |
| Backend API | Go + Fiber |
| ML Service | Python + FastAPI |
| Database | Postgres + pgvector |
| Queue | Redis + Asynq |
| TMDB Proxy | Cloudflare Worker (bypasses Indian ISP blocking) |

## Project Structure

```
framed/
├── apps/
│   └── web/                    # Next.js frontend (8 screens)
├── services/
│   ├── api/                    # Go backend
│   │   ├── cmd/server/         # Entry point — HTTP server + Asynq worker
│   │   ├── internal/
│   │   │   ├── handlers/       # HTTP route handlers
│   │   │   ├── scraper/        # Letterboxd RSS scraping
│   │   │   ├── enrichment/     # TMDB API client
│   │   │   ├── matching/       # pgvector similarity queries
│   │   │   ├── llm/            # Claude API (soul portraits, match blurbs)
│   │   │   └── jobs/           # Asynq background job workers
│   │   ├── pkg/
│   │   │   ├── config/         # Env config
│   │   │   ├── db/             # pgx connection pool
│   │   │   └── models/         # Domain structs + job payload types
│   │   └── migrations/         # SQL migrations (006 applied)
│   └── ml/                     # Python embedding service (FastAPI)
├── packages/
│   └── types/                  # Shared TypeScript types
└── infra/
    └── docker-compose.yml      # Postgres (pgvector) + Redis
```

---

## API Reference

Base URL: `http://localhost:8080` (local dev)

### Onboarding

#### `POST /onboard`
Kicks off a Letterboxd scrape for a user. Returns immediately with a job ID — processing happens in the background.

**Request**
```json
{
  "user_id": "00000000-0000-0000-0000-000000000001",
  "letterboxd_handle": "adityad1405"
}
```

**Response** `202 Accepted`
```json
{
  "job_id": "587340f2-2d79-439f-964c-aed143094161",
  "status": "queued"
}
```

---

#### `GET /status/:jobId`
Polls the status of a scrape job. Poll this until `status` is `complete`.

**Response** `200 OK`
```json
{
  "id": "587340f2-2d79-439f-964c-aed143094161",
  "status": "enriching",
  "films_found": 156,
  "films_processed": 152,
  "started_at": "2026-05-27T01:43:00Z",
  "completed_at": null
}
```

| `status` | Meaning |
|---|---|
| `queued` | Job created, not yet picked up |
| `scraping` | Fetching ratings from Letterboxd |
| `enriching` | Fetching film metadata from TMDB |
| `complete` | All done |
| `failed` | Something went wrong |

---

### Matching

#### `POST /match`
Computes taste-vector similarity matches for a user against all other users and persists them. Also enqueues background generation of "why you'd click" blurbs via Claude.

**Request**
```json
{ "user_id": "00000000-0000-0000-0000-000000000001" }
```

**Response** `200 OK`
```json
{
  "count": 3,
  "matches": [
    {
      "id": "match-uuid",
      "score": 0.87,
      "why_text": "",
      "status": "pending",
      "other_user": {
        "id": "other-user-uuid",
        "letterboxd_handle": "cinephile42",
        "age": 26,
        "location_city": "Mumbai",
        "location_country": "IN"
      },
      "first_date_film": {
        "id": "film-uuid",
        "title": "Mulholland Drive",
        "year": 2001
      }
    }
  ]
}
```

---

#### `GET /matches/:user_id`
Returns all stored matches for a user, enriched with the other user's profile and the suggested first-date film.

**Response** `200 OK` — same shape as the `matches` array above.

---

#### `GET /match/:matchId?user_id=...`
Returns full detail for a single match: the match header plus shared loved films and films where you disagreed.

**Response** `200 OK`
```json
{
  "id": "match-uuid",
  "score": 0.87,
  "why_text": "You both love slow-burn psychological dramas...",
  "status": "pending",
  "other_user": { ... },
  "first_date_film": { "id": "...", "title": "...", "year": 2001 },
  "shared_films": [
    { "id": "...", "title": "Parasite", "year": 2019, "your_rating": 5.0, "their_rating": 4.5 }
  ],
  "disagreement_films": [
    { "id": "...", "title": "Interstellar", "year": 2014, "your_rating": 5.0, "their_rating": 2.0 }
  ]
}
```

---

### Soul Portrait

#### `POST /soul/:user_id`
Generates a "Cinematic Soul" portrait for a user using Claude. Requires a taste vector to exist (i.e. matching must have run first). Stores the portrait and returns it immediately.

**Response** `200 OK`
```json
{
  "portrait": "You are drawn to films that exist in liminal spaces..."
}
```

---

#### `GET /soul/:user_id`
Returns a previously generated soul portrait. Returns `404` if none exists yet.

**Response** `200 OK`
```json
{
  "portrait": "You are drawn to films that exist in liminal spaces...",
  "summary_line": "You are drawn to films that exist in liminal spaces.",
  "computed_at": "2026-05-27T02:15:00Z"
}
```

---

## Background Pipeline

`POST /onboard` triggers this chain. Each step is an Asynq job.

```
scrape:profile
  ├─ Scrapes Letterboxd RSS for all ratings
  ├─ Upserts films + user_ratings into Postgres
  └─ Enqueues one enrich:film per film (deduplicated by task ID)

enrich:film  (one per film, shared catalogue — runs once globally)
  ├─ Fetches full metadata from TMDB (genres, directors, keywords, runtime, etc.)
  ├─ Skips if embedding_computed = true (already processed)
  ├─ Updates films table
  └─ Enqueues embed:film

embed:film
  ├─ Sends film text (genres + directors + themes + synopsis) to ML service
  ├─ Stores 256-dim vector in films.embedding
  ├─ Sets embedding_computed = true
  └─ When all of a user's films are embedded → enqueues vector:compute

vector:compute
  └─ Builds weighted average taste vector from all rated films
     (rating deviation from mean as weight)
     → stored in taste_vectors with top genres + directors

blurbs:generate  (triggered by POST /match)
  └─ Calls Claude Haiku to write "why you'd click" copy per match pair
     → stored in matches.why_text
```

---

## Database Schema

| Table | Purpose |
|---|---|
| `users` | Profile, preferences, location, seeking/intentions |
| `films` | Shared catalogue — enriched once, reused across all users |
| `user_ratings` | Raw Letterboxd ratings per user |
| `taste_vectors` | Weighted average embedding + top genres/directors per user |
| `soul_portraits` | AI-generated cinematic personality portrait |
| `matches` | Pairs with similarity score, first-date film, and why_text |
| `scrape_jobs` | Pipeline status tracking (scrape → enrich progress) |

Key columns on `films`:
- `embedding vector(256)` — pgvector embedding for similarity search
- `embedding_computed boolean` — definitive "fully processed" signal used to skip re-enrichment
- `directors text[]`, `genres text[]`, `themes text[]` — `NOT NULL DEFAULT '{}'`

Key columns on `scrape_jobs`:
- `films_found` — total films scraped from Letterboxd
- `films_processed` — enrich tasks actually queued
- `films_enriched` — running counter, incremented as each task finishes
- `enrichment_started_at` — stamped when first enrich task picks up

---

## Getting Started

### Prerequisites
- Go 1.22+
- Node.js 20+
- Python 3.11+
- Docker + Docker Compose

### 1. Clone and configure

```bash
git clone https://github.com/your-username/framed
cd framed
cp services/api/.env.example services/api/.env
# Fill in: TMDB_API_KEY, ANTHROPIC_API_KEY
```

### 2. Start infrastructure

```bash
make dev
# Starts Postgres (with pgvector) and Redis via Docker Compose
```

### 3. Run migrations

```bash
make db-migrate
```

### 4. Start the API + workers

```bash
make dev-api
# Go API on :8080, Asynq workers running in same process
```

### 5. Start the ML embedding service

```bash
cd services/ml
pip install -r requirements.txt
uvicorn main:app --port 8001
```

### 6. Start the frontend

```bash
make dev-web
# Next.js on :3000
```

---

## Key Commands

```bash
make dev          # start postgres + redis (docker)
make dev-api      # start Go API with hot reload
make dev-web      # start Next.js frontend
make db-migrate   # run pending SQL migrations
make db-flush     # wipe all data + reseed test users (local dev only)
make test-api     # run Go tests
make tidy         # install all dependencies
```

---

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `DATABASE_URL` | Yes | Postgres connection string |
| `REDIS_URL` | Yes | Redis address (e.g. `localhost:6379`) |
| `JWT_SECRET` | Yes | JWT signing secret |
| `TMDB_API_KEY` | Yes | TMDB v3 API key |
| `ANTHROPIC_API_KEY` | Yes | Claude API key (soul portraits + match blurbs) |
| `TMDB_BASE_URL` | No | Override TMDB base URL — set to Cloudflare Worker proxy when running in India to bypass ISP blocking of TMDB's CloudFront CDN |
| `ML_SERVICE_URL` | No | Embedding service URL (default: `http://localhost:8001`) |

---

## Notes

**TMDB proxy**: Indian ISPs RST TCP connections to TMDB's CloudFront nodes. A Cloudflare Worker at `https://tmdb-proxy.adityad1405.workers.dev/3` proxies requests through Cloudflare's backbone. Set `TMDB_BASE_URL` to this value in local dev. Leave unset in production deployments outside India.

**Enrichment idempotency**: `enrich:film` tasks are deduplicated by Asynq task ID (`enrich:<film_uuid>`). A film that already has `embedding_computed = true` is skipped immediately. Re-running `/onboard` for the same user is safe.

**Matching**: Uses pgvector cosine similarity between taste vectors. The "first date film" is selected as the film nearest to the midpoint of both users' taste vectors that neither has seen.
