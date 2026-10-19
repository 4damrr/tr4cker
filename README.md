# tr4cker

Personal finance tracker: keep spending and life a bit more mindful.

tr4cker is built around one question — **is this financial period still on
plan?** — rather than a historical spending report. Every page is built around
what is left, not what has already happened.

## Stack

- Go `net/http` backend
- HTMX frontend with HTML templates rendered by Go
- SQLite (`modernc.org/sqlite`, no CGO)

No SPA framework, no ORM, no router library, no build step.

## Pages

| Page            | Path             | What it does                                                        |
| --------------- | ---------------- | ------------------------------------------------------------------- |
| **Dashboard**   | `/`              | Current period: total balance, per-account balances, income, spending, remaining planned amount, plan progress, recent transactions |
| **Transactions**| `/transactions`  | List and filter history; edit and soft-delete                        |
| **New entry**   | `/transactions/new` | Fast dedicated entry view, defaulted to an expense on the most recently used account |
| **Planning**    | `/planning`      | Planned amount per category with actual spending, remaining amount and progress |
| **To Buy**      | `/to-buy`        | Intended purchases with priority, quantity and expected price         |
| **Accounts**    | `/accounts`      | Accounts and their balances, plus category management in a secondary section |

## Financial periods

A period runs from the **25th of one month through the 24th of the next**, and is
labelled by the month it ends in:

```text
25 Sep – 24 Oct 2026
```

A transaction dated 2026-10-05 belongs to `25 Sep – 24 Oct 2026`; one dated
2026-10-25 belongs to the next period.

## Money

Amounts are stored as floating-point numbers and rounded to **two decimal
places** whenever they are written. Entry rounds rather than rejecting, so
`25,000.505` becomes `25000.51`. Display shows a decimal part only when it is
not zero:

```text
Rp 900,000      whole
Rp 900,000.50   fractional
```

Account balances are rounded on every write, so a balance always holds at most
two decimals and the dashboard total equals the sum of the accounts it lists.

## How planned amounts are measured

- **`needs` and `wants` categories are spending limits.** Remaining is planned
  minus actual spending and may go negative; going over shows as *overspent*.
  Only expense transactions count.
- **`savings` categories are targets.** Progress is measured from transfers into
  the category, including transfers flagged as investments. Expenses and income
  never count toward a savings target.

A transfer between two of your own accounts is never spending.

## Layout

```
cmd/server/            entrypoint
internal/db/           SQLite connection and migration
internal/money/        amount parsing, two-decimal rounding, formatting
internal/period/       25th-to-24th period arithmetic
internal/store/        repositories: accounts, categories, transactions, planning, to-buy
internal/web/          HTTP handlers and view models
templates/layout.html  shared shell and navigation
templates/pages/       one template per page
templates/components/  HTMX partial regions
static/css             styles
static/js              client libraries (HTMX, vendored)
data/                  local SQLite file (created at runtime)
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

## Starter data

A fresh database is seeded with a few accounts (`BRI`, `Cash`, `Gopay`,
`ShopeePay`, all at balance 0) and categories across all three buckets:

| Bucket   | Categories                                  |
| -------- | ------------------------------------------- |
| `needs`  | Food, Transport, Bills                       |
| `wants`  | Entertainment, Shopping                      |
| `savings`| Emergency, Investment                        |

Deactivate anything you do not use from the Accounts page.

## Resetting the database

The database file is local and gitignored, so the simplest reset is to delete
it and let the migration recreate the schema and starter data on next start:

```bash
rm data/tr4cker.db data/tr4cker.db-shm data/tr4cker.db-wal
make run
```

There is no data migration path from earlier versions: the boilerplate sample
`expenses` table is dropped rather than converted.

## Tests

```bash
go build ./... && go test ./...
```

The suite covers amount rounding and formatting, period arithmetic, the
repositories, balance updates, and the HTTP handlers and templates.
