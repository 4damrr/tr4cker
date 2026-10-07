# Proposal

## Why

Typical money-tracking apps show historical spending but not whether the current financial period is still on plan. This change turns the existing Go/HTMX/SQLite boilerplate into a personal tracker that makes period progress visible in real time and keeps recording a transaction fast enough to use after a purchase.

## What Changes

- **BREAKING**: Replace the sample `expenses` table and home-page expense CRUD with the domain schema and pages defined in `openspec/specs/database-design.md` and `openspec/specs/frontend-design.md`.
- Add SQLite tables for `transactions`, `category`, `account`, `planning`, and `to_buy`, including soft delete where those documents specify it.
- Record income, expense, and transfer transactions with category, accounts, timestamp, and an investment flag; keep transfers out of spending totals.
- Manage money-holding accounts with stored balances that update in the same SQLite transaction as related money movements.
- Classify categories into needs, wants, and savings buckets shared by transactions, planning, and to-buy items.
- Plan spending per category for the application financial period (25th–24th) and compare planned amount, actual spending, remaining amount, and progress.
- Track a to-buy list (name, category, priority, quantity, expected unit price, URL, notes, status) without auto-creating a transaction when an item is marked bought.
- Replace the current single “Home / Expenses” screen with dashboard, transactions, planning, accounts, and to-buy pages, using HTMX partial updates for common mutations.
- Provide fast transaction entry with defaults and few required fields.

## Capabilities

### New Capabilities

- `transactions`: Record, list, filter, edit, and soft-delete income, expense, and transfer movements, including fast entry.
- `categories`: Maintain shared category keys and needs/wants/savings buckets used by transactions, planning, and to-buy items.
- `accounts`: Maintain money-holding accounts, notes, stored balances, and deactivation via soft delete.
- `monthly-planning`: Define planned amounts per category for the 25th–24th financial period and compare them to actual spending.
- `to-buy-list`: Track intended purchases and buying status without automatically posting a transaction.
- `dashboard`: Show the current period overview (income, spending, remaining plan, progress, recent transactions).

### Modified Capabilities

- None. `openspec list --specs` reports no existing capabilities. Informal notes in `openspec/specs/` (`Proposal.md`, `database-design.md`, `frontend-design.md`) are design constraints, not OpenSpec capability specs.

## Impact

- **Schema**: `internal/db/db.go` migrations replace `expenses` (`amount REAL`) with the five domain tables and integer IDR amounts.
- **HTTP**: `internal/web/` grows from `GET /`, `POST /expenses`, and `DELETE /expenses/{id}` to page and HTMX routes for dashboard, transactions, categories (as needed for forms), accounts, planning, and to-buy.
- **UI**: `templates/` and `static/css/app.css` replace the expense form/list with multi-page layout and navigation; keep Go templates + vendored HTMX, no SPA framework.
- **Data**: Existing local `data/tr4cker.db` expense rows are not migrated; this is a local personal app still in boilerplate.
- **Dependencies**: Stay on Go `net/http`, `html/template`, and `modernc.org/sqlite`. No new services or build pipeline.
- **Auth / multi-user**: Out of scope; single local user assumed.
