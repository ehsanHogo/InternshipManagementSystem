# Internship Management System

## Stack

- Go, Gin, GORM, PostgreSQL
- Angular, TypeScript, SCSS, PrimeNG, PrimeIcons
- Docker and Docker Compose

## Prerequisites

- Docker with Docker Compose
- Node.js 20.19+ or 22.12+
- npm

Go is only required when running or building the backend outside Docker.

## Run the project

From this directory, start PostgreSQL and the backend:

```bash
docker compose up --build
```

In another terminal, install and start the Angular development server:

```bash
cd frontend
npm install
npm start
```

Open the login page at <http://localhost:4200/login>. The backend is available at <http://localhost:8082>, and its health endpoint is:

```text
GET http://localhost:8082/api/health
```

Expected response:

```json
{ "status": "ok" }
```

The Angular development server proxies `/api` requests to the backend. The backend CORS policy separately permits only `http://localhost:4200` by default.

## Authentication

The backend stores bcrypt password hashes and issues one HMAC-SHA256 access JWT after a successful login. Tokens contain the user ID, role, email, issue time, and expiry. The Angular client stores the demo token in `localStorage`, adds it to authenticated API requests, and validates an existing token against `/api/auth/me` whenever the application starts.

The available roles are `STUDENT`, `PROFESSOR`, `COMPANY_SUPERVISOR`, `UNIVERSITY_SUPERVISOR`, and `ADMIN`. Each user has one role. There are deliberately no refresh tokens, role/permission tables, or password recovery flow in this MVP.

These development environment variables configure token signing:

```text
JWT_SECRET=replace-with-a-long-random-secret
JWT_EXPIRES_HOURS=24
UPLOAD_DIR=uploads
```

Docker Compose supplies a development-only secret. Replace it outside local demos. When running the backend directly, use the values in `backend/.env.example` as a guide; the Go application reads environment variables but does not load the file automatically.

## Demo accounts

The backend runs GORM `AutoMigrate` and an idempotent seed on startup. Each account is looked up by email before insertion, so restarting the backend does not create duplicates. All five accounts use the explicitly non-production password `Demo123!`.

| Role                  | Email                   | Name            | Additional details                               |
| --------------------- | ----------------------- | --------------- | ------------------------------------------------ |
| Student               | `student@demo.local`    | علی رضایی       | Student number `40123456`, major مهندسی کامپیوتر |
| Professor             | `professor@demo.local`  | دکتر محمد احمدی | —                                                |
| University supervisor | `university@demo.local` | کارشناس آموزش   | —                                                |
| Company supervisor    | `company@demo.local`    | رضا محمدی       | —                                                |
| Admin                 | `admin@demo.local`      | مدیر سیستم      | —                                                |

After login, the frontend redirects to <http://localhost:4200/dashboard>. Visiting the dashboard without a valid authenticated session redirects back to the login page. Logging out removes the stored token.

## Test authentication from the command line

Log in and copy the returned `token` value:

```bash
curl -X POST http://localhost:8082/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"student@demo.local","password":"Demo123!"}'
```

Use that token to request the current user:

```bash
curl http://localhost:8082/api/auth/me \
  -H 'Authorization: Bearer YOUR_TOKEN'
```

The same header can be used with `GET /api/protected`, the small authenticated verification endpoint. Missing, malformed, expired, or incorrectly signed tokens receive HTTP 401. Invalid login credentials also receive HTTP 401 without revealing which credential was wrong.

Stop the Docker services with:

```bash
docker compose down
```

PostgreSQL data remains in the `postgres_data` Docker volume. To also remove that development data, explicitly run `docker compose down --volumes`.
Final report PDFs are stored on local disk. Docker Compose persists them in the `final_reports` volume and the database stores only protected metadata and the generated storage name.

## Active internship reporting

Milestone 5 keeps an internship case in `ACTIVE` while the student submits up to eight weekly reports and a PDF final report. The assigned company supervisor can confirm each weekly report once and submit one read-only final company evaluation. Final-report downloads require authentication and access to the related internship case.

Student pages:

- `/student/weekly-reports`
- `/student/final-report`

Company reporting is available inside each assigned ACTIVE case at `/company/internships/:id`.

## Useful development commands

```bash
# Build the backend locally
cd backend
go build ./cmd/api

# Run backend tests
go test ./...

# Build the frontend
cd frontend
npm run build

# View service logs
docker compose logs backend postgres
```

## VS Code tasks

When this repository is open as the VS Code workspace, run tasks from **Terminal → Run Task**. The most useful entries are:

- `Setup: Install Dependencies` for first-time setup.
- `Development: Start` to start PostgreSQL/backend in Docker and Angular locally.
- `Database: Up`, `Database: Drop`, or `Database: Open psql` for database work. `Database: Drop` removes only PostgreSQL data; pgAdmin and its saved settings are preserved.
- `Build: All` to build both applications.
- `Backend: Test` and `Health: Check Backend` for quick verification.
- `Docker: Follow Logs`, `Docker: Show Status`, and `Docker: Stop` for container management.

`Ctrl+Shift+B` runs the default `Build: All` task.

## Troubleshooting

- Docker Compose exposes PostgreSQL on host port `5433` and the backend on `8082` to avoid common conflicts with locally installed services. Inside Docker, they still use ports `5432` and `8080`.
- If port `5433`, `8082`, or `4200` is already in use, stop the conflicting local service before starting this project.
- If the frontend shows that the server is unavailable, check `docker compose ps`, then inspect `docker compose logs backend postgres`.
- The backend waits for PostgreSQL's health check before starting. If it exits, its log includes the database connection error rather than continuing without a database.
- When running the backend directly instead of with Docker, copy values from `backend/.env.example` into your shell environment and ensure PostgreSQL is reachable at `localhost:5433`.
