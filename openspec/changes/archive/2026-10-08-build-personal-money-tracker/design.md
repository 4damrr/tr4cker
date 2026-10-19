# Design

## Context

See `proposal.md` - Why for motivation and - What Changes for scope.

The starting point is a deliberately small boilerplate: one `expenses` table with a `REAL` amount, three routes (`GET /`, `POST /expenses`, `DELETE /expenses/{id}`), four templates, vendored HTMX, 113 lines of CSS, and no tests. Constraints that shape the approach:

- `internal/db/db.go` opens SQLite with `foreign_keys=1`, `journal_mode=WAL`, `busy_timeout=5000`, and `SetMaxOpenConns(1)`. The single connection is what makes multi-statement transactions safe without a busy-retry layer, and it must be preserved.
- `internal/web/server.go` parses templates by explicit file list with `ParseFiles`, and every page currently defines `{{define "content"}}`. One shared block name cannot survive a second page.
- Every money column becomes a floating-point value (`openspec/specs/database-design.md` - Money Representation), with every amount rounded to two decimal places on write and the decimal part omitted on display when it is zero. The 25th–24th period replaces calendar months for all planning maths.
- Authoritative constraints: `openspec/specs/database-design.md` and `openspec/specs/frontend-design.md`. Behaviour is specified in `specs/*/spec.md`.

## Goals / Non-Goals

**Goals:**

- One migration path from the `expenses` table to the five domain tables, with no data migration for existing local rows.
- Period arithmetic, money formatting, and aggregate queries expressed once, so the dashboard, planning page, and transaction list cannot disagree about what a period contains.
- Balance updates that are atomic with the transaction that caused them, on a single-connection pool.
- Testable pure logic, since the repository currently has no tests at all.

**Non-Goals:**

- Authentication, multi-user support, or any notion of shared access.
- Import/export, recurring transactions, budgets across multiple periods, or reporting.
- Any client-side framework, router, ORM, migration library, or build tooling. Standard library plus `modernc.org/sqlite` only.
- Charts. Progress is communicated with numbers and bars.

## Decisions

### The financial period is labelled by its ending month

A period covers the 25th of month M-1 through the 24th of month M, and is stored as `year = M's year, month = M`. "25 Oct - 24 Nov 2026" is therefore `2026/11`.

*Why:* `planning` already stores `month` + `year` with a unique constraint on `(month, year, category)`; labelling by the ending month means the stored key is a normal calendar month, so the existing uniqueness rule and ordinary date arithmetic both work unchanged. Labelling by the starting month would make a stored month mean "the month that begins here", which reads backwards next to a calendar picker.

*Alternative considered:* a separate period index (period 1 = 25 Dec - 24 Jan). Rejected - it disconnects the stored value from the calendar month the user actually thinks in, and adds a lookup dimension for no benefit at this scale.

### Account references are plain text, validated in the application

`transactions.account_from` and `transactions.account_to` carry **no** foreign key. `external` is a reserved literal that is never a row in `account`. `transactions.category`, `planning.category`, and `to_buy.category` keep their foreign key to `category(key)`.

*Why:* the design document's own examples use `account_from = external`, so either the FK is dropped or `external` becomes a real account row that leaks into every account list and balance total. Validation is type-shaped rather than referential: an expense needs an active `account_from` and `account_to = external`, an income needs an active `account_to` and `account_from = external`, a transfer needs two active accounts. That is stricter than an FK would be, and it is checkable before any write.

*Alternative considered:* seeding `external` as a hidden account row with balance 0. Rejected - it makes "total balance across accounts" a query that must remember to exclude it.

*Trade-off:* a mistyped account key is possible at the SQL level. All writes go through the same repository helpers, and a test asserts that an unknown non-`external` key is rejected.

### `account.key` is immutable; balance and notes are editable

*Why:* the key is the identifier, and transactions reference it as text. Renaming would need to cascade into `transactions.account_from` and `transactions.account_to` to stay truthful, which is three write paths for a field a personal user almost never changes. Balance and notes cover the realistic edits, including correcting a starting balance.

*Alternative considered:* allow rename with a cascading update. Rejected as disproportionate complexity.

### Money is floating-point, rounded to two decimals on every write

Every money column is REAL and every Go money value is `float64`. One rule covers every money path: an amount is rounded to two decimal places whenever it is written, and rounded to two decimal places again when it is formatted.

- **Entry.** A typed amount is rounded rather than rejected, so `25,000.505` becomes `25000.51` and `0.756` becomes `0.76`. The user is never blocked for precision, and there is no second code path for "manual" versus "computed" amounts.
- **Balances.** `account.balance` is rounded in the same statement that changes it, so a balance at rest holds at most two decimals and the dashboard's total is exactly the sum of the balances it lists.
- **Totals.** Period income, spending, and remaining amounts are summed from already-rounded stored amounts and rounded once more at display, so `900000.0000000001` reads as `900,000`.

*Display trims a zero decimal part:* `900000` renders as `900,000` and `900000.50` renders as `900,000.50`. Two decimals only appear when there is something to show.

*Why:* floating-point storage is a deliberate reversal of the design document's original integer recommendation, confirmed by the user, and fractional rupiah is carried end to end rather than discarded.

*Alternative considered:* round balances to whole rupiah while keeping fractional entries. Rejected - sub-rupiah precision is then progressively destroyed, so three `0.25` expenses move a balance by `1` instead of `0.75`, and the balance stops agreeing with the sum of its own transactions.

*Alternative considered:* store integer rupiah and keep decimals only in the UI. Rejected at the same point, for the same reason.

*Trade-off:* rounding to a fixed number of decimal places is not associative. Three successive `0.33` transfers can land one cent away from `0.99`. Rounding on every write bounds the error rather than eliminating it, which is the accepted cost of fractional float money.

### Balances are stored and adjusted inside the same SQLite transaction

`account.balance` is the source of truth, updated on create, edit, and soft-delete of a transaction. Edit reverses the old effect before applying the new one; soft-delete reverses the effect and keeps the row.

*Why:* the design document answers both open questions "yes". Storing the balance keeps account listing a single-column read instead of a running aggregate, and `SetMaxOpenConns(1)` means a multi-statement `sql.Tx` needs no retry loop. All three operations share one helper pair (`apply`, `reverse`) so the sign rules live in exactly one place.

### Savings progress counts transfers, including investments

For a category in the `savings` bucket, actual progress is the sum of `type = transfer` amounts in that category dated within the period, with no exclusion for `is_investment`. Income and expenses never contribute.

*Why:* savings is a movement of money into a designated place, not a purchase, so the expense aggregate is the wrong measure. Including investments means a contribution to a savings account and a contribution to an investment account both read as progress, which is what a user saving on a goal expects. The design document settles that investments are transfers rather than a separate type, so they are already `type = transfer` with the flag set.

*Consequence:* the same query shape serves both buckets with one differing predicate - `type = 'expense'` for `needs`/`wants`, `type = 'transfer'` for `savings`. That predicate is a function of the category's bucket, not of the caller, so planning and dashboard cannot pick different ones.

### Progress and remaining amount are derived per bucket

For `needs`/`wants`: remaining = planned − actual, which may go negative and is rendered as an overspent state; progress = actual / planned. For `savings`: progress = actual / planned toward the target, with no remaining amount once the target is met. A category with actual spending and no planned amount shows the actual and no remaining or progress.

*Why:* this keeps one number pair per category rather than a general-purpose "plan vs actual" abstraction with a mode flag threaded through templates and queries.

### Templates get a per-page content block plus one shared shell

`layout.html` defines `layout` with a `{{block "content" .}}` call; each page file defines its own uniquely named block and is rendered through `{{template "<block>" .}}`. A shared `renderPage(w, r, "dashboard.html", "dashboard-content", data)` helper wires the two together.

*Why:* the current single `{{define "content"}}` collides the moment a second page exists. Go's template package offers no clean per-render block override, so the unique-block-name convention is the smallest workable fix and stays inside the standard library. Templates move to `templates/pages/*.html` and are discovered by `template.ParseFS` over an embedded or on-disk `templates/` tree rather than an explicit file list, so adding a page is a file drop rather than a code edit.

*Alternative considered:* a full-page template per view with duplicated `<head>`/nav markup. Rejected - duplication is exactly what the shell prevents.

### Transaction entry is a dedicated view, not a modal or inline form

`GET /transactions/new` is a full page reached from the dashboard and the transactions list. It defaults to type `expense` with the current time and the most recently used account preselected, so the common case is amount plus submit.

*Why:* the frontend design lists entry as a main area and explicitly asks that entry not require navigating through multiple pages. A dedicated view needs no overlay JavaScript, works without HTMX at all, and is bookmarkable on mobile. An inline form on the transactions page was the alternative; it pushes the entry form above the list the user came to read and has no room for the account and category pickers.

### HTMX replaces mutated regions, never the whole page

Mutations post to the same routes and return the affected region: the transaction list, the dashboard figures, one planning row. Redirect-after-post is used for full page loads; `HX-Redirect` is not needed because each mutation renders its own target region.

*Why:* the frontend design asks for server-rendered HTML over client-side state. Vendored HTMX is already in `static/js/htmx.min.js`, and the existing `hx-on::after-request` reset trick is the only custom script in the app and stays.

### Route layout follows `net/http` method-and-wildcard patterns

Page routes return full pages; mutation routes are the same paths with the method that changes them, and the handler detects an HTMX request by header to decide page vs region.

*Why:* Go 1.22+ pattern routing (`POST /transactions/{id}`, `DELETE /transactions/{id}`) is already used for `DELETE /expenses/{id}` and removes the need for a router dependency.

## Risks / Trade-offs

- **Dropping the account foreign key lets a bad key reach the database** -> All writes go through repository helpers that resolve account keys first; a test asserts an unknown non-`external` key is rejected, and account choices in forms come only from active accounts.
- **Editing an account's balance directly can silently disagree with its transactions** -> Accepted, and it is the intended escape hatch for a wrong starting balance. Editing is a deliberate user action on the accounts page, not something a transaction triggers.
- **Stored balances drift if any write path bypasses the helpers** -> Mitigated by keeping every transaction mutation in one repository file with one pair of apply/reverse helpers, and by a test that records, edits, then soft-deletes a transaction and asserts balances return to their starting values.
- **`ParseFS` template discovery changes the current explicit-list behaviour** -> The `WEB_ROOT` override still works, and a test renders every page template to catch parse errors at test time rather than at first request.
- **Six page templates plus shared components is the largest single piece of work** -> Sequenced behind the data layer so each page lands against working queries, and ordered so the shell and dashboard come before the secondary pages.
- **Single connection serialises all writes** -> Fine for a single local user; noted as the reason no retry layer is needed rather than as a scaling concern.
- **Rounding to two decimals is not associative, so balances can drift by a cent** -> Every write rounds, which bounds the error instead of letting it accumulate. A test records, edits, and soft-deletes a transaction repeatedly and asserts the balance returns to its starting value, which catches drift introduced by the edit and reverse paths specifically. The residual risk on long sequences of sub-cent amounts is accepted, since `database-design.md` was revised to float at the user's direction.
- **Savings progress counting investments is a judgement call** -> Recorded as a confirmed decision rather than an open question; it is isolated to one predicate in the planning aggregate so reversing it later touches one place.

## Migration Plan

1. Replace the `expenses` migration with the five domain table definitions and drop `expenses`.
2. Delete `internal/web/expenses.go`, the expense handlers, and the expense templates.
3. Ship the pages against the new schema.

Existing rows in `data/tr4cker.db` are not migrated; the file is local, still boilerplate sample data, and is gitignored. Rollback is `rm data/tr4cker.db` and restart - `migrate` recreates the schema. If a stale `expenses` table is present after upgrade it is dropped, not renamed, so the reset path is the only supported recovery.

## Open Questions

- **Primary navigation layout.** The frontend design leaves the exact layout open. This design assumes a single top bar with Dashboard, Transactions, Planning, To Buy, and a secondary link from Accounts to Categories. Cheap to restyle; no spec or task depends on the exact layout.
- **Whether the planning page lists all categories or only those with plans.** This design assumes it lists every active category, showing "no plan set" for unplanned ones, so a category can be planned in place. Adding a toggle is cosmetic.
- **Recent-transaction row count on the dashboard.** Assumed small and fixed. Changing it is a template constant.
