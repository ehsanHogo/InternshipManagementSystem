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
- University supervisor: manages users/companies/assignments, selects the placement and records introduction-letter metadata, and views submitted company details.
- Company supervisor: recruits students through opportunity applications and submits start/placement details for the university-selected official case.
- Professor: reviews assigned active/completed cases and records the final qualitative result when every prerequisite is complete.
- Admin: retains a deliberately minimal dashboard and receives no workflow mutation permissions.

Frontend route guards improve navigation UX; backend role middleware and case-ownership checks remain the authorization boundary.

## Workflow

The statuses are:

```text
OpportunityApplication: PENDING → ACCEPTED (company recruitment)

InternshipCase:
DRAFT → PENDING_UNIVERSITY_REVIEW
      → PENDING_COMPANY_DETAILS
      → PENDING_FINAL_APPROVAL

PENDING_UNIVERSITY_REVIEW → CANCELLED (university cancellation)
```

Recommended demonstration order:

1. Register a company, publish an opportunity, and accept the student's opportunity application.
2. As the student, create the official case, enter credits/mobile, select one to three accepted applications in priority order, and submit.
3. As the university supervisor, select one preference and enter introduction-letter number/date. The selected opportunity determines the company supervisor automatically.
4. As that company supervisor, open **پرونده‌های کارآموزی**, enter subject, Jalali start date, actual workplace address/phone, and confirm submission to university.
5. Refresh the company case and inspect the student and university views. Placement details are read-only in **در انتظار تأیید نهایی آموزش**.

There is no second company acceptance after the introduction letter. University final approval/correction and activation are deferred to later milestones. Existing legacy reporting/evaluation code is retained; this milestone adds no reporting or completion actions.

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
