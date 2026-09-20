# Internship Management System

## Project Overview

A Persian, RTL internship workflow for students, university supervisors, company supervisors, professors, and a minimal administrator role. The application preserves the complete record as a case advances from draft through university/company approval, active reporting, final evaluations, and completion.

## Technology Stack

- Angular 20, TypeScript, SCSS, PrimeNG, PrimeIcons, Vazirmatn
- Go 1.24, Gin, GORM
- PostgreSQL 16
- Docker Compose and Nginx

## Prerequisites

For the all-Docker setup, install Docker with Docker Compose. For local development, also install Node.js 20.19+ or 22.12+, npm, and Go 1.24+.

## Environment Configuration

Docker Compose reads optional values from a root `.env` file. Copy `.env.example` and replace the development-only defaults when needed:

```bash
cp .env.example .env
```

The backend supports `APP_PORT`, `FRONTEND_ORIGIN`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`, `JWT_SECRET`, `JWT_EXPIRES_HOURS`, and `UPLOAD_DIR`. See `backend/.env.example` for local-backend defaults. The Go process reads environment variables directly and does not load `.env` files itself.

Host ports can be changed with `POSTGRES_PORT`, `BACKEND_PORT`, `FRONTEND_PORT`, and `PGADMIN_PORT`. Never use the included demo database password, pgAdmin password, or JWT secret in production.

## Docker Setup

Start PostgreSQL, the API, the Angular/Nginx frontend, and pgAdmin:

```bash
docker compose up --build
```

Open:

- Application: <http://localhost:4200>
- API health check: <http://localhost:8082/api/health>
- pgAdmin: <http://localhost:5050>

The frontend sends relative `/api` requests. In Docker, Nginx proxies them to the backend service; in local development, Angular's `proxy.conf.json` proxies them to `http://localhost:8082`. No machine-specific API URL is compiled into the application.

PostgreSQL data, pgAdmin settings, and uploaded final reports use the `postgres_data`, `pgadmin_data`, and `final_reports` named volumes.

## Local Development

Start infrastructure and run the backend locally:

```bash
docker compose up -d postgres pgadmin
cd backend
APP_PORT=8082 DB_HOST=localhost DB_PORT=5433 go run ./cmd/api
```

In another terminal:

```bash
cd frontend
npm ci
npm start
```

Open <http://localhost:4200/login>. Restart `go run` after backend changes. Database and uploaded-file volumes survive backend restarts.

## Main Roles

- Student: creates and submits an application, maintains unconfirmed weekly reports, uploads/replaces the final PDF while active, and views the completed result.
- University supervisor: manages users/companies/assignments and advances submitted cases through university workflow stages.
- Company supervisor: confirms assigned placements and weekly reports, then submits one final company evaluation.
- Professor: reviews assigned active/completed cases and records the final qualitative result when every prerequisite is complete.
- Admin: retains a deliberately minimal dashboard and receives no workflow mutation permissions.

Frontend route guards improve navigation UX; backend role middleware and case-ownership checks remain the authorization boundary.

## Workflow

The statuses are:

```text
DRAFT
→ PENDING_UNIVERSITY_APPROVAL
→ PENDING_COMPANY_APPROVAL
→ COMPANY_APPROVED
→ UNIVERSITY_APPROVED
→ ACTIVE
→ COMPLETED
```

Recommended demonstration order:

1. Sign in as the student, create the application, add one to three preferences, and submit it.
2. Sign in as the university supervisor, select a preference and company supervisor, enter letter details, and send the case to the company.
3. Sign in as the company supervisor and confirm placement details. A student-proposed company is approved and added to the company table only at this point.
4. Sign in as the university supervisor, approve the company-confirmed case, and activate it.
5. Sign in as the student and submit weeks 1–8 plus a final PDF.
6. Sign in as the company supervisor, confirm all eight reports, and submit the company evaluation.
7. Sign in as the professor, review the historical record and final PDF, choose `عالی`, `خوب`, or `مردود`, optionally comment, and complete the case.
8. Sign in as the student and verify the completed status, final result, comment, reports, and final file remain visible.

## Excel Import Format

Only `.xlsx` files up to 10 MB are accepted. Header spelling is Persian; header order is flexible.

Student import headers:

```text
نام و نام خانوادگی
ایمیل
شماره دانشجویی
رشته
```

Professor import headers:

```text
نام و نام خانوادگی
ایمیل
```

Each successful row returns a temporary password. It is shown only in that result and cannot be retrieved later, so copy it before leaving the page.

## File Upload Rules

- Final reports must be non-empty PDF files no larger than 10 MB.
- The server validates extension and MIME type and generates a unique storage name.
- Files are downloaded only through an authenticated, case-authorized endpoint.
- Filesystem paths and stored filenames are not exposed by the API.
- A student may replace a final report only while the case is `ACTIVE`; completed cases are immutable.

## How to Run

The shortest demo command is:

```bash
docker compose up --build
```

Useful verification commands:

```bash
cd backend
go test ./...
go vet ./...
go build ./...

cd ../frontend
npm ci
npm run build
```

To apply idempotent migrations/seeds to the local Docker database without deleting data:

```bash
cd backend
DB_HOST=localhost DB_PORT=5433 go run ./cmd/migrate
```

## How to Stop

Stop containers while retaining persistent data:

```bash
docker compose down
```

To intentionally remove the database, pgAdmin state, and uploaded reports as well:

```bash
docker compose down --volumes
```

The second command is destructive and should not be used when demo data must be preserved.

## Troubleshooting

- Ports used on the host are `4200` (frontend), `8082` (backend), `5433` (PostgreSQL), and `5050` (pgAdmin). Stop any conflicting local service or change the port mapping.
- Run `docker compose ps` and `docker compose logs backend postgres frontend` if the UI cannot reach the API.
- The backend waits for PostgreSQL health; the frontend waits for backend health.
- A 401 clears the local session and returns the browser to login. Sign in again if a demo JWT expires.
- Local frontend development requires the backend on port `8082`, matching `frontend/proxy.conf.json`.
- Uploaded reports missing after a container restart usually indicates the `final_reports` volume was removed.
