# Spec Delta

## Purpose

Gives the user one screen that shows where they stand in the current financial period: how much money they hold, what came in and went out this period, and how much of the plan is left.

## ADDED Requirements

### Requirement: The dashboard is the default screen
The system SHALL serve the dashboard at the site root, so opening the application shows the dashboard without the user choosing a page.

#### Scenario: Open the application
- **WHEN** the user opens the application root address
- **THEN** the dashboard is shown

#### Scenario: Reach the dashboard by navigation
- **WHEN** the user chooses Dashboard from the main navigation
- **THEN** the dashboard for the current financial period is shown

### Requirement: Show the current financial period
The system SHALL show which 25th-to-24th financial period the dashboard describes, labelled with the month the period ends in.

#### Scenario: Show the period containing today
- **WHEN** the user opens the dashboard on 2026-10-28
- **THEN** the dashboard shows the period 25 Oct – 24 Nov 2026

### Requirement: Show total and per-account balances
The system SHALL show the total balance across active accounts and a list of those accounts with their individual balances.

#### Scenario: Total across accounts
- **WHEN** active accounts `BRI` and `Gopay` hold 2500000 and 500000
- **THEN** the dashboard shows a total balance of 3000000 and lists `BRI` at 2500000 and `Gopay` at 500000

#### Scenario: Deactivated accounts stay out of the balance
- **WHEN** account `Cash` holding 150000 has been deactivated
- **THEN** `Cash` is not listed and its 150000 is not part of the total balance

### Requirement: Displayed money figures are rounded and trimmed
The system SHALL round displayed amounts to two decimal places and SHALL omit the decimal part when it is zero.

#### Scenario: Float noise is not shown
- **WHEN** a period's spending accumulates to 900000.0000000001
- **THEN** the dashboard shows spending as 900,000 with no decimal part

#### Scenario: A fractional total keeps its decimal part
- **WHEN** a period's spending accumulates to 900000.5
- **THEN** the dashboard shows spending as 900,000.50

#### Scenario: Balance total matches the listed balances
- **WHEN** active accounts hold 2500000 and 500000
- **THEN** the total balance of 3000000 equals the sum of the listed per-account balances

### Requirement: Show period income and spending
The system SHALL show total income and total spending for the current period from active transactions dated within that period, and MUST NOT count transfers as spending.

#### Scenario: Income and spending for the period
- **WHEN** the period 25 Oct – 24 Nov 2026 holds 5000000 income and 900000 expenses
- **THEN** the dashboard shows income 5000000 and spending 900000 for that period

#### Scenario: Transfers are not spending
- **WHEN** the period holds a 100000 transfer between two active accounts
- **THEN** spending for that period is unchanged by it

#### Scenario: Transactions from other periods are excluded
- **WHEN** an expense is dated 2026-11-02 and today is 2026-10-28
- **THEN** it is not counted in the current period's spending

### Requirement: Show the remaining planned amount and planning progress
The system SHALL show the total remaining planned amount across the current period's categories, and each planned category's progress.

#### Scenario: Remaining planned amount
- **WHEN** the period plans 1500000 for `Food` with 900000 spent and 500000 for `Transport` with nothing spent
- **THEN** the dashboard shows a remaining planned amount of 1100000

#### Scenario: Overspending reduces the remaining amount below zero
- **WHEN** the period plans 500000 for `Food` with 900000 spent
- **THEN** `Food` contributes -400000 to the remaining planned amount and is marked as overspent

#### Scenario: Period with no plans
- **WHEN** the current period has no planned amounts
- **THEN** the dashboard says there is no plan for this period rather than showing a progress total

### Requirement: Show recent transactions
The system SHALL show the most recently dated active transactions with their date, amount, type, category, and accounts.

#### Scenario: Most recent first
- **WHEN** transactions are dated 2026-10-20, 2026-10-28, and 2026-10-15
- **THEN** they are listed in the order 2026-10-28, 2026-10-20, 2026-10-15

#### Scenario: Soft-deleted transactions are not listed
- **WHEN** a transaction has been soft-deleted
- **THEN** it does not appear among recent transactions

### Requirement: Dashboard figures refresh after a change
The system SHALL refresh the dashboard's period figures, balances, and recent transactions without a full page reload after the user records, edits, or deletes a transaction or changes a planned amount.

#### Scenario: Recording a transaction updates the dashboard
- **WHEN** the user records a 25000 expense from the dashboard area and the dashboard figures are shown again
- **THEN** spending, remaining planned amount, and the recent transactions include that expense

#### Scenario: Deleting a transaction updates the dashboard
- **WHEN** the user soft-deletes a transaction and the dashboard figures are shown again
- **THEN** spending, balances, and the recent transactions no longer include it
