# Problem Log

Small API to track competitive programming problems (Codeforces style). You save problems, log your attempts
(AC, WA, TLE...) and ask it what to redo next.

Stack: Go (chi router + pgx, no ORM), PostgreSQL 17, everything started with Docker Compose.

## What is submitted

- this GitHub repo: source code, `Dockerfile`, `compose.yaml`, this README (API documentation is below) and the Postman collection `problem-log.postman_collection.json`
- a zip with the real `.env` file (database password), it is **not** in Git
- the repo only has `.env.example` with placeholder values

## Run it

Unzip the submitted zip, open a terminal in the folder where the `.env` file is, then copy and paste all of this:

```bash
git clone https://github.com/TheILICH/problemlog.git
cp .env problemlog/.env
cd problemlog
docker compose up --build
```

It downloads the project, puts the `.env` file inside it, and starts the app. The first start takes a minute or two.
The app is ready when the logs show `listening on :8080`.

No `.env` file? Replace the second line with `cp .env.example .env` and run it after the `cd problemlog` line.

The database is not empty on the first start, the migrations insert 8 demo problems (ids 1-8).

## Test with Postman

1. Open Postman, press `Ctrl + O` (or the **Import** button) and choose `problem-log.postman_collection.json` from the repo
2. The collection "Problem Log" appears on the left with 6 folders, all requests go to `http://localhost:8080`
3. Send the requests from top to bottom (or click the collection → **Run collection** to send all of them at once)

**Create problem** saves the id of the new problem into the collection variable `{{problemId}}`,
so all the next requests work on that problem, no need to change ids by hand.
It also adds a small random suffix to `external_id`, so the collection can be run many times without `409`.

| Folder | Request | Method and URL | What you should see |
|---|---|---|---|
| 1. Basics | Index (list of routes) | `GET /` | `200`, list of all routes |
| | Health | `GET /health` | `200`, `{"db": "up", "status": "ok"}` |
| 2. Problems CRUD | List problems | `GET /problems` | `200`, the 8 demo problems |
| | List problems (tag + status filter) | `GET /problems?tag=dp&status=attempted` | `200`, only attempted dp problems |
| | List problems (rating range) | `GET /problems?min_rating=1200&max_rating=1800` | `200`, only problems in this range |
| | Create problem | `POST /problems` | `201`, new problem with status `todo` |
| | Create same problem again (409) | `POST /problems` | `409`, same `platform` + `external_id` |
| | Get problem by id | `GET /problems/{{problemId}}` | `200`, the problem you just created |
| | Update problem (PATCH) | `PATCH /problems/{{problemId}}` | `200`, rating, tags and notes changed, title same |
| 3. Attempts | Add attempt (WA) | `POST /problems/{{problemId}}/attempts` | `201`, `problem_status` is now `attempted` |
| | Add attempt (AC) | `POST /problems/{{problemId}}/attempts` | `201`, `problem_status` is now `solved` |
| | List attempts of problem | `GET /problems/{{problemId}}/attempts` | `200`, 2 attempts, newest (AC) first |
| 4. Stats and review | Stats | `GET /stats` | `200`, solved/total per tag and per rating bucket |
| | Review (what to redo) | `GET /review` | `200`, problems to redo |
| 5. Error cases | Get unknown problem (404) | `GET /problems/999999` | `404` |
| | Bad id in path (400) | `GET /problems/abc` | `400` |
| | Create with rating out of range (400) | `POST /problems` with `rating: 5000` | `400` |
| | PATCH with typo in field (400) | `PATCH` with `"ratting"` | `400`, unknown field |
| | Attempt with unknown verdict (400) | `POST` attempt with `verdict: "OK"` | `400` |
| 6. Cleanup | Delete problem | `DELETE /problems/{{problemId}}` | `204` |
| | Get deleted problem (404) | `GET /problems/{{problemId}}` | `404`, it is really gone |

## API documentation

Base URL: `http://localhost:8080`. All bodies are JSON.

Errors always look like this, with the right status code:

```json
{ "error": "rating must be between 800 and 3500" }
```

| Code | When |
|---|---|
| `400` | bad JSON, unknown field (a typo like `"ratting"`), bad id, value out of range |
| `404` | problem with this id does not exist, or unknown route |
| `409` | problem with the same `platform` + `external_id` already exists |
| `503` | `/health` only, database is down |

Rules for the values:

- `rating`: 800-3500, optional
- `status`: `todo`, `attempted` or `solved`
- `verdict`: `AC`, `WA`, `TLE`, `MLE`, `RE` or `CE`
- `tags`: max 10, each max 40 characters, saved lowercase without duplicates
- `minutes`: 0-1440

### Problem object

```json
{
  "id": 9,
  "platform": "codeforces",
  "external_id": "1352C",
  "title": "K-th Not Divisible by n",
  "rating": 1200,
  "tags": ["math", "binary search"],
  "status": "todo",
  "notes": "",
  "created_at": "2026-10-31T07:40:12.345678Z"
}
```

### GET /health

Checks the database connection.

- `200` → `{"db": "up", "status": "ok"}`
- `503` → `{"db": "down", "status": "degraded"}`

### GET /problems

List of problems, sorted by id. All filters are optional and can be combined:

| Query param | Example | Meaning |
|---|---|---|
| `tag` | `dp` | problems that have this tag |
| `status` | `attempted` | only this status |
| `min_rating` | `1200` | rating >= 1200 |
| `max_rating` | `1800` | rating <= 1800 |

Example: `GET /problems?tag=dp&status=attempted&min_rating=1200&max_rating=1800`

- `200` → array of problem objects (empty array if nothing matches)
- `400` → bad `status` or a rating that is not a number

### POST /problems

Adds a problem. `external_id` and `title` are required, `platform` is `codeforces` if not sent.
A new problem always starts with status `todo`.

```json
{
  "external_id": "1352C",
  "title": "K-th Not Divisible by n",
  "rating": 1200,
  "tags": ["math", "binary search"],
  "notes": "optional text"
}
```

- `201` → the created problem object
- `400` → missing or invalid field
- `409` → same `platform` + `external_id` already saved

### GET /problems/{id}

- `200` → one problem object
- `400` → id is not a positive number
- `404` → no problem with this id

### PATCH /problems/{id}

Changes only the fields you send, the others stay the same. Allowed fields: `title`, `rating`, `tags`, `status`, `notes`.

```json
{
  "rating": 1300,
  "notes": "answer is k + (k-1)/(n-1)"
}
```

- `200` → the updated problem object
- `400` → invalid value or unknown field
- `404` → no problem with this id

### DELETE /problems/{id}

Deletes the problem and all its attempts (`ON DELETE CASCADE`).

- `204` → deleted, empty body
- `404` → no problem with this id

### POST /problems/{id}/attempts

Logs one attempt. `attempted_at` is optional (for logging old attempts), without it the current time is used.

```json
{
  "verdict": "WA",
  "minutes": 35
}
```

The attempt insert and the status change happen in one transaction:

- `AC` → problem becomes `solved`
- any other verdict → `todo` becomes `attempted`, `solved` stays `solved`

Response `201`:

```json
{
  "id": 12,
  "problem_id": 9,
  "verdict": "WA",
  "minutes": 35,
  "attempted_at": "2026-10-31T07:45:03.120934Z",
  "problem_status": "attempted"
}
```

- `400` → bad verdict or minutes
- `404` → no problem with this id

### GET /problems/{id}/attempts

- `200` → array of attempts, newest first
- `404` → no problem with this id

### GET /stats

Counted in SQL with `GROUP BY` and `FILTER`, rating buckets are 200 wide.

```json
{
  "total": 9,
  "solved": 4,
  "by_tag": [
    { "tag": "dp", "solved": 1, "total": 3 }
  ],
  "by_rating": [
    { "bucket": "1200-1399", "solved": 2, "total": 3 }
  ]
}
```

### GET /review

Problems worth redoing: the last attempt failed, or the last attempt is an `AC` older than 14 days.
Failed ones come first, then the oldest solves.

```json
[
  {
    "id": 9,
    "platform": "codeforces",
    "external_id": "1352C",
    "title": "K-th Not Divisible by n",
    "rating": 1300,
    "reason": "last attempt failed (WA)"
  }
]
```

## Data stays after the containers are removed

The database files are in the named volume `pgdata`, so the data is not inside the container.

1. In Postman send **Create problem**
2. Stop and remove the containers, the volume stays:
   ```bash
   docker compose down
   docker compose up -d
   ```
3. In Postman send **Get problem by id**, the problem is still there

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

Only the backend port is published. The database has no `ports`, it is reachable only from the backend
through the `backend-net` network, by the service name `db`.

## Project structure

```
compose.yaml                       backend + db services, network, named volume, healthcheck
Dockerfile                         builds the Go backend image
.env.example                       placeholders only, copy it to .env
.gitignore / .dockerignore         keep .env out of Git and out of the image
problem-log.postman_collection.json   Postman requests for all endpoints
cmd/server/main.go                 startup: config -> connect -> migrate -> HTTP server
internal/config/                   reads env variables, builds the database URL
internal/database/                 pgx connection pool (with retry) + goose migrations
internal/store/                    all the SQL queries (plain SQL, no ORM)
internal/api/                      chi router, validation, JSON handlers
migrations/                        00001_init.sql (tables), 00002_seed_demo.sql (demo data)
```
