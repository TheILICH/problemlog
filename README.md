# Problem Log

Small API to track competitive programming problems (Codeforces style). You save problems, log your attempts
(AC, WA, TLE...) and ask it what to redo next.

Stack: Go (chi router + pgx, no ORM), PostgreSQL 17, everything started with Docker Compose.

## How this submission works

- the code is in this Git repo, the repo link is in `link.txt`
- the real `.env` (database password) is **not** in Git, it is next to `link.txt`
- the repo only has `.env.example` with placeholder values

## What you need

- Git
- Docker with Compose v2 (the `docker compose` command, with a space), for example Docker Desktop
- internet on the first start (it downloads the images and Go modules)
- port 8080 free (or set another `APP_PORT` in `.env`)

## Run it

```bash
git clone https://github.com/TheILICH/problemlog.git
cd problemlog
```

Now copy the `.env` file from the submission into this `problemlog` folder (the one with `compose.yaml`), then:

```bash
docker compose up --build
```

The app is ready when the logs show `listening on :8080`. In a second terminal `docker compose ps` should show
both services as **healthy**.

Everything in one line, if the `.env` file is in your current folder (bash, zsh, Git Bash, PowerShell 7):

```bash
git clone https://github.com/TheILICH/problemlog.git && cp .env problemlog/.env && cd problemlog && docker compose up --build
```

No `.env` file? Then `cp .env.example .env` also works, the placeholder values are fine for a first start.

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

Windows: run `curl` in Git Bash or WSL (the PowerShell `curl` is something else), or use Postman or the browser.

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

Add a problem:

```bash
curl -s -X POST localhost:8080/problems -H "Content-Type: application/json" \
  -d '{"external_id":"1352C","title":"K-th Not Divisible by n","rating":1200,"tags":["math","binary search"]}'
```

Log an attempt (use the `id` from the answer above, it is `9` on a fresh database):

```bash
curl -s -X POST localhost:8080/problems/9/attempts -H "Content-Type: application/json" \
  -d '{"verdict":"WA","minutes":35}'
```

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

## How the pieces connect

- `compose.yaml` starts two services, `db` (postgres:17-alpine) and `backend` (built from `./Dockerfile`)
- both are on the network `backend-net`, the backend finds the database by the service name `db`
- the db port is not published, only the backend can talk to it. The API is published on `localhost:8080`
- `depends_on: condition: service_healthy`: the backend waits until `pg_isready` passes, and it also retries
  the connection for 60 s as a backup
- the credentials live only in `.env`. Compose reads it and passes the values to both services
- on start the backend runs the SQL files from `migrations/` (goose, embedded in the binary): tables first,
  then the demo data, one time only

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

## Assignment checklist

| Requirement | Where |
|---|---|
| Backend with a meaningful API | `internal/api`, table above |
| Relational database, backend reads and writes | PostgreSQL, `internal/store` |
| `compose.yaml` with backend and database | `compose.yaml` |
| Custom Dockerfile for the backend | `Dockerfile` |
| Ports, networks, service names, dependencies | `backend-net`, `db` / `backend`, `depends_on: service_healthy` |
| Environment variables for DB and app config | `compose.yaml`, read in `internal/config` |
| No secrets in source or Git | only `.env.example` is committed, `.env` is in `.gitignore` |
| Named volume for DB data | `pgdata` |
| Data stays after stop and recreate | section "Data stays after the containers are removed" |
| Starts with `docker compose up` | section "Run it" |

## Troubleshooting

- **`required variable POSTGRES_PASSWORD is missing`**: there is no `.env` next to `compose.yaml`
  (copy the one from the submission, or `cp .env.example .env`).
- **`port is already allocated`**: another program uses 8080, put another `APP_PORT` in `.env`.
- **Backend can't log in after the password in `.env` was changed**: Postgres reads the password only when it
  creates the volume the first time. Run `docker compose down -v` and start again.
- **Logs**: `docker compose logs -f backend`
- **Unit tests** (needs Go installed): `go test ./...`
