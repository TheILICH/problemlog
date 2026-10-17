# Problem Log

Small API to track competitive programming problems (Codeforces style). You save problems, log your attempts
(AC, WA, TLE...) and ask it what to redo next.

Stack: Go (chi router + pgx, no ORM), PostgreSQL 17, everything started with Docker Compose.

## How this submission works

- the code is in this Git repo, the repo link is in `link.txt`
- the real `.env` (database password) is **not** in Git, it is next to `link.txt`
- the repo only has `.env.example` with placeholder values

## Run it

Open a terminal in the folder where the `.env` file is (the submission folder), then copy and paste all of this:

```bash
git clone https://github.com/TheILICH/problemlog.git
cp .env problemlog/.env
cd problemlog
docker compose up --build
```

It downloads the project, puts the `.env` file inside it, and starts the app. The first start takes a minute or two.
The app is ready when the logs show `listening on :8080`. Then open http://localhost:8080/health

No `.env` file? Replace the second line with `cp .env.example .env` and run it after the `cd problemlog` line.

## Check that it works

Open http://localhost:8080/health in the browser, you should see:

```json
{ "db": "up", "status": "ok" }
```

The database is not empty on the first start, the migrations insert 8 demo problems:

```bash
curl -s localhost:8080/problems
curl -s "localhost:8080/problems?tag=dp&status=attempted"
curl -s localhost:8080/stats
curl -s localhost:8080/review
```

## API

| Method | Path | What it does |
|---|---|---|
| GET | `/health` | Pings the database, `200` when up, `503` when down |
| GET | `/problems` | List, filters: `?tag=dp&status=attempted&min_rating=1200&max_rating=1800` |
| POST | `/problems` | Add a problem |
| GET | `/problems/{id}` | One problem (`404` if unknown) |
| PATCH | `/problems/{id}` | Change `title`, `rating`, `tags`, `status`, `notes` (send only what changes) |
| DELETE | `/problems/{id}` | Delete the problem and its attempts (`204`) |
| GET | `/problems/{id}/attempts` | Attempt history, newest first |
| POST | `/problems/{id}/attempts` | Log an attempt, also updates the problem status in one transaction |
| GET | `/stats` | Solved/total per tag and per rating bucket, counted in SQL (`GROUP BY`) |
| GET | `/review` | Problems to redo: last attempt failed, or solved more than 14 days ago |

Rules: `rating` is 800-3500, `verdict` is one of `AC WA TLE MLE RE CE`, status is `todo`, `attempted` or `solved`.
An `AC` attempt makes the problem `solved`, any other verdict moves `todo` to `attempted`.
The same `(platform, external_id)` twice gives `409`. Bad input gives `400` with `{"error": "..."}`.

## Data stays after the containers are removed

The database files are in the named volume `pgdata`, so the data is not inside the container.

```bash
docker compose down            # removes containers and network, keeps the volume
docker compose up -d           # new containers, same data
curl -s localhost:8080/problems/9      # the problem you added is still there
```

To start from zero (volume is deleted, demo data comes back):

```bash
docker compose down -v
docker compose up -d
```

## Configuration

| Variable | Where | Meaning |
|---|---|---|
| `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` | `.env` | Database credentials, used by both services |
| `APP_PORT` | `.env` | Port on your machine for the API (default 8080) |
| `DB_HOST`, `DB_PORT` | `compose.yaml` | Where the backend finds the database (`db`, `5432`) |
| `PORT` | `compose.yaml` | Port the Go server listens on inside the container |

## Project structure

```
compose.yaml              backend + db services, network, named volume, healthchecks
Dockerfile                builds the Go backend image
.env.example              placeholders only, copy it to .env
.gitignore / .dockerignore   keep .env out of Git and out of the image
cmd/server/main.go        startup: config -> connect -> migrate -> HTTP server
internal/config/          reads env variables, builds the database URL
internal/database/        pgx connection pool (with retry) + goose migrations
internal/store/           all the SQL queries (plain SQL, no ORM)
internal/api/             chi router, validation, JSON handlers
migrations/               00001_init.sql (tables), 00002_seed_demo.sql (demo data)
```

