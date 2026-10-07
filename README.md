# tr4cker

Personal finance tracker: keep spending and life a bit more mindful.

## Stack

- Go `net/http` backend
- HTMX frontend with HTML templates
- SQLite (`modernc.org/sqlite`, no CGO)

## Layout

```
cmd/server/          entrypoint
internal/db/         SQLite connection and migrations
internal/web/        HTTP handlers
templates/           layout and pages
templates/components HTMX partials
static/css           styles
static/js            client libraries (HTMX)
data/                local SQLite file (created at runtime)
```

## Run locally

From the repo root:

```bash
make run
```

Or:

```bash
chmod +x scripts/run.sh
./scripts/run.sh
```

Then open [http://localhost:8080](http://localhost:8080).

Optional environment variables:

- `ADDR` — listen address (default `:8080`)
- `DB_PATH` — SQLite file (default `data/tr4cker.db`)
- `WEB_ROOT` — directory that contains `templates/` and `static/` (default `.`)

`make dev` runs without writing a binary. `make build` writes `bin/tr4cker`.
