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
- University supervisor: manages students, professors, assignments, and manual university company trust approval; selects the placement and records introduction-letter metadata, and views submitted company details.
- Company supervisor: manages registration information while pending/rejected; after Admin registration approval, recruits students through opportunity applications and submits start/placement details for the university-selected official case.
- Professor: reviews assigned active/completed cases and records the final qualitative result when every prerequisite is complete.
- Admin: reviews company registrations at `/admin/company-registrations`; approves or rejects pending registrations without changing university trust.

Frontend route guards improve navigation UX; backend role middleware and case-ownership checks remain the authorization boundary.

## Workflow

The statuses are:

```text
OpportunityApplication: PENDING → ACCEPTED (company recruitment)

InternshipCase:
DRAFT → PENDING_UNIVERSITY_REVIEW
PENDING_UNIVERSITY_REVIEW → REVISION_REQUESTED → PENDING_UNIVERSITY_REVIEW
PENDING_UNIVERSITY_REVIEW → PENDING_COMPANY_DETAILS → PENDING_FINAL_APPROVAL
PENDING_FINAL_APPROVAL → PENDING_COMPANY_DETAILS (company correction)
PENDING_FINAL_APPROVAL → ACTIVE → PASSED / FAILED
PENDING_UNIVERSITY_REVIEW → CANCELLED (university cancellation)
Any unfinished case → CANCELLED (atomic internship term closure)
```

Recommended demonstration order:

Before creating an official case, sign in as the university supervisor and open an academic internship term at `/university/internship-terms`.

1. Register a company, sign in as Admin and approve its registration, then publish an opportunity and accept the student's opportunity application.
2. As the student, create the official case, enter credits/mobile, select one to three accepted applications in priority order, and submit.
3. As the university supervisor, select one preference and enter introduction-letter number/date. The selected opportunity determines the company supervisor automatically.
4. As that company supervisor, open **پرونده‌های کارآموزی**, enter subject, Jalali start date, actual workplace address/phone, and confirm submission to university.
5. As the university supervisor, request company placement correction if needed, then give final approval. The case becomes **ACTIVE** immediately, including when its declared start date is in the future.
6. Continue the existing weekly reporting and evaluation workflows.

If none of the submitted preferences is suitable, the university can request student preference revision with a required reason. The student edits and resubmits the same case, preserving its term and professor. Students may apply for new opportunities in **DRAFT** or **REVISION_REQUESTED**; successful **PASSED** history still permanently blocks recruitment.

There is no separate manual activation action. On startup or `cmd/migrate`, existing **READY_TO_START** records are converted to **ACTIVE** without changing unrelated case data.

## Student Company Ratings \

After successful completion, a student can open a **PASSED** case in their case history and submit a final company rating from 1 to 5 stars. This also works for older cases and closed academic terms. **FAILED**, **CANCELLED**, and unfinished cases cannot be rated. Each case allows one rating; there are no edit/delete actions or comments.

`POST /api/student/internship-cases/:id/rating` accepts only `{"rating": 4}`. The server verifies ownership and resolves the company through the case's selected preference and accepted opportunity application. The database enforces a unique case, the 1–5 range, foreign keys, and a company index. Startup or `cmd/migrate` adds the ratings table without backfilling ratings.

Student opportunity list/detail show `companyAverageRating` and `companyRatingCount`. These are company-wide aggregates, so all opportunities from one company show the same result. A company without ratings returns `null`/`0` and displays «هنوز ارزیابی‌ای ثبت نشده است». University company details and case review, and assigned professor case details, show the aggregate read-only. Company responses and Admin registration review contain no rating fields. Ratings do not change application eligibility, completion, term handling, registration status, or manual university approval.

## Company Registration Governance

Self-registration creates one company and its linked company supervisor with `RegistrationStatus=PENDING` and `IsApproved=false`. Pending and rejected supervisors can log in, edit registration data, and view status/rejection reasons at `/company/profile`. Rejected companies explicitly resubmit the same record for review; editing alone does not change status. Operational routes require Admin registration approval on the server and in frontend navigation/guards.

`RegistrationStatus` (`PENDING`, `APPROVED`, `REJECTED`) controls access to company business features. `IsApproved` still means university trust: a passed internship makes a company eligible, and the university manually approves it. Students can apply and the university can select companies with approved registration even when `IsApproved=false`. The student approved-companies directory still uses only university trust.

The university can inspect companies and existing supervisors but cannot create either. Existing companies are backfilled as registration-approved without changing trust. New demo companies are operational; subsequent startup preserves real decisions and existing trust values. Historical cases remain readable.

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
- Initial final report upload requires an `ACTIVE` case and all distinct weeks 1–8 submitted and approved by both the company reviewer and professor.
- The server validates extension and MIME type and generates a unique storage name.
- Files are downloaded only through an authenticated, case-authorized endpoint.
- Filesystem paths and stored filenames are not exposed by the API.
- A student may replace a final report only while the case is `ACTIVE` and the report is `REVISION_REQUESTED`. Corrections do not revalidate initial weekly readiness; completed cases are immutable.

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

Enable the full PostgreSQL integration suite against a dedicated test database:

```bash
cd backend
TEST_DATABASE_DSN='host=localhost port=5433 user=postgres password=postgres dbname=internship_test sslmode=disable' go test -p 1 ./... -count=1
```

The test database must already exist. Run packages serially (`-p 1`) because existing integration tests share the public schema and run GORM migrations. Term and compatibility tests also create and remove isolated schemas.

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
