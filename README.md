# Internship Management System

A small Milestone 1 foundation for a university internship management application. The current scope provides a Persian RTL Angular page and a Go health API backed by a PostgreSQL connectivity check. Authentication and internship business entities are intentionally not included yet.

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

Open the frontend at <http://localhost:4200>. The backend is available at <http://localhost:8082>, and its health endpoint is:

```text
GET http://localhost:8082/api/health
```

Expected response:

```json
{"status":"ok"}
```

The Angular development server proxies `/api` requests to the backend. The backend CORS policy separately permits only `http://localhost:4200` by default.

Stop the Docker services with:

```bash
docker compose down
```

PostgreSQL data remains in the `postgres_data` Docker volume. To also remove that development data, explicitly run `docker compose down --volumes`.

## Useful development commands

```bash
# Build the backend locally
cd backend
go build ./cmd/api

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
- `Database: Start` or `Database: Open psql` for database work.
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
