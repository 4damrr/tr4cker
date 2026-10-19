# Tasks

## 1. Schema and shared primitives

- [x] 1.1 Replace the `expenses` migration in `internal/db/db.go` with the five domain tables (`transactions`, `category`, `account`, `planning`, `to_buy`) using floating-point IDR amounts in REAL columns, `CHECK` constraints for `trx_type`, `buckets`, `priority`, and `buying_status`, `deleted_at` on the three soft-deletable tables, `UNIQUE(year, month, category)` on `planning`, a foreign key from the three `category` columns to `category(key)`, and **no** foreign key on `account_from`/`account_to`; drop `expenses` and verify `sqlite3 data/tr4cker.db ".schema"` shows all five tables with REAL money columns
- [x] 1.2 Add an `internal/money` package that parses, rounds to two decimal places, and formats floating-point amounts, and verify table tests cover `25000`, `25000.50`, `0.75` staying `0.75`, `0.756` rounding to `0.76`, `25000.505` rounding to `25000.51`, `900000.0000000001` formatting as `900,000` with no decimal part, `900000.5` formatting as `900,000.50`, grouping separators such as `1500000` to `1,500,000`, a negative remaining amount such as `-0.60`, and rejection of empty and non-numeric input
- [x] 1.3 Add an `internal/period` package that maps a date to its 25th-to-24th financial period as the ending `year`/`month`, returns the period's inclusive date bounds, and renders the label; verify table tests cover the 24th and 25th of a month, 31-day and 30-day months, the December-to-January year boundary, and the label `25 Oct - 24 Nov 2026` for October and November 2026
- [x] 1.4 Seed a small starter set of accounts and categories covering all three buckets, and verify a fresh `data/tr4cker.db` comes up with `needs`, `wants`, and `savings` categories represented and the app starts

## 2. Accounts and categories

- [x] 2.1 Implement the account repository (create, list active, update balance and notes, deactivate) against `account`, and verify repository tests assert that a deactivated account is excluded from active lists while its balance and history remain
- [x] 2.2 Implement the category repository (create, list active, update, deactivate) against `category`, and verify repository tests assert bucket values are constrained to `needs`, `wants`, `savings` and that deactivation hides the key from new-record choices without removing the row
- [x] 2.3 Add the accounts page and its routes (list, create, edit, deactivate) with HTMX partial updates, and verify handler tests cover successful create, a rejected duplicate key, and deactivate returning the refreshed list region
- [x] 2.4 Add category management as a secondary section reached from the accounts page, and verify handler tests cover creating a category in each bucket and deactivating one so it disappears from the choices

## 3. Transactions and balances

- [x] 3.1 Implement the transaction repository (create, list with filters, get, update, soft-delete) against `transactions`, and verify repository tests cover listing only non-deleted rows and filtering by type, category, account, and date range
- [x] 3.2 Implement the balance apply/reverse helpers so a create, edit, and soft-delete each adjust the affected accounts by the right sign and round the stored balance to two decimal places, and verify a test that a 100000 balance minus a 25000.50 expense stores 74999.50, that a 0 balance minus a 0.756 expense stores -0.76, and that recording, editing, then soft-deleting a transaction leaves every balance back at its starting value
- [x] 3.3 Wrap each transaction write and its balance updates in one SQLite transaction, and verify a test that forces a failure after the first balance update rolls back the balance change and stores no transaction
- [x] 3.4 Validate account references in the application layer - expense needs an active source and `external` destination, income the reverse, transfer two active accounts - and verify a test rejects an unknown non-`external` account key
- [x] 3.5 Add the transactions page with the list, filtering, and edit routes using HTMX partial updates, and verify handler tests cover filtered listing, editing an amount, and soft-delete returning the refreshed list
- [x] 3.6 Add the dedicated transaction entry view at `/transactions/new` defaulting to type `expense`, the current time, and the most recently used account, and verify handler tests confirm the defaults render and that a save with no amount is rejected

## 4. Monthly planning

- [x] 4.1 Implement the planning repository with an upsert keyed on `(year, month, category)` and optional notes, and verify a repository test that saving a second amount for the same category in the same period leaves exactly one row
- [x] 4.2 Implement the actual-spending aggregate for a category and period, selecting expenses for `needs`/`wants` and transfers for `savings` including investment-flagged ones, filtering on the period bounds and excluding soft-deleted rows; verify aggregate tests cover transfer exclusion, investment inclusion for savings, soft-deleted exclusion, and an amount dated outside the period
- [x] 4.3 Compute remaining amount and progress per bucket, rounding the displayed remaining amount to two decimal places, including negative remaining and overspent for `needs`/`wants`, target progress for `savings`, and the unplanned-category case; verify table tests cover within limit, overspent on 500000 planned against 500000.60 spent showing a remaining amount of -0.60, a planned amount of 1500000.756 stored as 1500000.76, plan with no spending, spending with no plan, and a savings target met
- [x] 4.4 Add the planning page listing every active category for the selected period with planned, actual, remaining, and progress, plus previous/next period navigation and inline HTMX amount editing, and verify handler tests cover period navigation changing the figures and an edited amount updating its row

## 5. To-buy list

- [x] 5.1 Implement the to-buy repository (create, list with status filter, update, set status, delete) with `qty * price` as the expected total, and verify repository tests cover the total for multiple units and the three allowed statuses
- [x] 5.2 Add the to-buy page and routes with priority-ordered listing, status filter, add, edit, mark bought, cancel, and remove, and verify handler tests confirm a status change updates the row and that a rejected priority leaves the list unchanged
- [x] 5.3 Verify that changing an item's status creates no transaction and changes no account balance, including for an item whose category has spending in the current period

## 6. Dashboard

- [x] 6.1 Implement the dashboard query returning the current period label, total balance across active accounts, the per-account list, period income and spending, and recent active transactions, and verify a test asserts transfers are excluded from spending and rows dated outside the period are excluded
- [x] 6.2 Compute the remaining planned amount and per-category progress for the period from the planning aggregate, rounding displayed money to two decimal places, and verify a test covers a period with no plans reporting no plan rather than a zero total, a total accumulating to 899999.9999999999 displaying as 900,000 with no decimal part, and a total of 900000.5 displaying as 900,000.50
- [x] 6.3 Add the dashboard at `GET /` rendering period, total balance, per-account balances, income, spending, remaining planned amount, progress, and recent transactions, and verify handler tests assert each figure is displayed rounded to two decimal places with the decimal part omitted when it is zero, that the total balance equals the sum of the listed per-account balances, and that a soft-deleted transaction disappears from the list
- [x] 6.4 Wire the dashboard figures, balances, and recent transactions to refresh via HTMX after a transaction or planning change, and verify a handler test shows an updated figure after a recorded expense

## 7. Shell, navigation, and responsive layout

- [x] 7.1 Convert `layout.html` to a shell with a `{{block "content"}}` call and a per-page content block, switch template loading to `ParseFS` over `templates/` so pages are discovered rather than listed, and verify a test renders every page template and fails on a parse error
- [x] 7.2 Add primary navigation for Dashboard, Transactions, Planning, and To Buy with the active item marked, reachable from every page, and verify the dashboard and each page render their navigation
- [x] 7.3 Add the responsive styles for navigation, lists, tables, and progress bars to `static/css/app.css` reusing the existing custom properties and `.panel`/`.row` primitives, and verify the pages render correctly at a narrow viewport and a desktop width
- [x] 7.4 Remove the expense templates, `internal/web/expenses.go`, and the `/expenses` routes, and verify no route, template, or CSS selector referencing expenses remains

## 8. Documentation and full-app check

- [x] 8.1 Update `README.md` for the dashboard-first app: the new pages, the financial period convention, the seeded starter data, and the fact that the old sample database is reset by deleting `data/tr4cker.db`, and verify the documented run and reset commands work as written
- [x] 8.2 Run the full build and test suite with `go build ./... && go test ./...` and verify both pass
- [x] 8.3 Exercise the app end to end against a fresh database - seed accounts and categories, record income, expense, and transfer transactions, check balances, set plans for all three buckets, add to-buy items and mark one bought, and confirm the dashboard, planning, and to-buy pages agree with the recorded data and that marking an item bought created no transaction

## Workflow follow-up

- Archive the change once implementation is reviewed and accepted.
- Verify the archived result with `openspec validate --strict`.
