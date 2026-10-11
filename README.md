# Internship Management System

A Persian, right-to-left application for managing university internships, from placement applications and approvals to weekly reports and final evaluation.

## Features

- Role-based access for students, university supervisors, company supervisors, professors, and administrators.
- Internship opportunities, recruitment, placement approvals, and academic term management.
- Weekly reports, final PDF submissions, evaluations, and company ratings.
- Account and company verification, Excel imports, profiles, and notifications.

**Stack:** Angular 20 / PrimeNG · Go / Gin / GORM · PostgreSQL 16 · Docker Compose / Nginx.

## Quick start

Install Docker with Docker Compose, then run from the repository root:

```bash
cp .env.example .env
docker compose up --build
```

| Service | Default URL |
| --- | --- |
| Application | <http://localhost:4200> |
| API health | <http://localhost:8082/api/health> |
| pgAdmin | <http://localhost:5050> |

The backend applies migrations and creates missing demo accounts automatically. Newly created demo accounts use password `Demo123!`:

| Role | Email |
| --- | --- |
| Student | `student@demo.local` |
| University supervisor | `university@demo.local` |
| Company supervisor | `company@demo.local` |
| Professor | `professor@demo.local` |
| Admin | `admin@demo.local` |

To try the workflow, first sign in as the university supervisor and open an internship term.

Stop with `docker compose down`; database records and uploaded reports remain in Docker volumes. The included credentials and secrets are for development only; replace them before deployment.

## Local development

Requires Go 1.24+, npm, and Node.js 20.19+ (20.x), 22.12+ (22.x), or 24+.

Start PostgreSQL and the backend from the repository root:

```bash
docker compose up -d postgres
cd backend
APP_PORT=8083 DB_HOST=localhost DB_PORT=5433 go run ./cmd/api
```

In a second terminal, from the repository root:

```bash
cd frontend
npm ci
npm start
```

Open <http://localhost:4200>. The Angular development proxy expects the API on port `8083`. The Go process reads environment variables directly; it does not load `.env` files. For custom database settings, export matching values from [backend/.env.example](backend/.env.example).

## Checks

Run from the repository root:

```bash
(cd backend && go test -p 1 ./... && go vet ./... && go build ./...)
(cd frontend && npm ci && npm run build)
```

Database integration tests require `TEST_DATABASE_DSN` pointing to an existing, dedicated test database; otherwise, they are skipped.

## Project layout

- [`frontend/`](frontend/) — Angular interface.
- [`backend/`](backend/) — Go API, database migrations, and tests.
- [`docker-compose.yml`](docker-compose.yml) — development services and persistent volumes.
