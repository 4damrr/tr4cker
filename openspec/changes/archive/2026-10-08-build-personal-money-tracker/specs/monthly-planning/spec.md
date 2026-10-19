# Spec Delta

## Purpose

Lets the user set a spending limit or savings target per category for each 25th-to-24th financial period, and see how far the recorded transactions have moved against it.

## ADDED Requirements

### Requirement: Financial periods run from the 25th to the 24th
The system SHALL treat each financial period as starting on the 25th of one calendar month and ending on the 24th of the next, and SHALL label a period with the month it ends in.

#### Scenario: The 25th opens a new period
- **WHEN** a transaction is dated 2026-10-25
- **THEN** it counts toward the period 25 Oct – 24 Nov 2026

#### Scenario: The 24th closes a period
- **WHEN** a transaction is dated 2026-10-24
- **THEN** it counts toward the period 25 Sep – 24 Oct 2026

#### Scenario: Period label names the ending month
- **WHEN** the user views the period 25 Oct – 24 Nov 2026
- **THEN** it is shown as `25 Oct – 24 Nov 2026`

### Requirement: One planned amount per category per period
The system SHALL keep at most one planned amount for a category within a period, and SHALL let the user create it and change it afterwards. A planned amount is a floating-point value rounded to two decimal places.

#### Scenario: Create a planned amount
- **WHEN** the user plans 1500000 for `Food` in the period 25 Oct – 24 Nov 2026
- **THEN** `Food` shows a planned amount of 1500000 for that period

#### Scenario: Change an existing planned amount
- **WHEN** the user plans 500000 for `Food` in a period that already has a planned amount of 1500000
- **THEN** that period shows 500000 for `Food` and still has exactly one entry for it

#### Scenario: Fractional planned amount
- **WHEN** the user plans 1500000.75 for `Food`
- **THEN** that period stores 1500000.75 for `Food`

#### Scenario: Finer input is rounded
- **WHEN** the user plans 1500000.756 for `Food`
- **THEN** that period stores 1500000.76 for `Food`

#### Scenario: Plan notes are optional
- **WHEN** the user saves a planned amount without notes
- **THEN** the entry is stored with no notes

### Requirement: Actual spending comes from expense transactions
The system SHALL derive actual spending for a category in a period from active expense transactions in that category dated within that period, and MUST NOT count transfers or income as spending.

#### Scenario: Expense counts toward its category
- **WHEN** the user records a 900000 expense in `Food` dated 2026-10-28
- **THEN** actual spending for `Food` in the period 25 Oct – 24 Nov 2026 is 900000

#### Scenario: Transfer is not spending
- **WHEN** the user records a 100000 transfer between two active accounts
- **THEN** no category in that period shows an increase in spending for it

#### Scenario: Soft-deleted expense stops counting
- **WHEN** the user soft-deletes the 900000 expense
- **THEN** actual spending for `Food` in that period is 0

#### Scenario: Spending stays in its own period
- **WHEN** the user records a 900000 expense in `Food` dated 2026-11-25
- **THEN** it counts toward the period 25 Nov – 24 Dec 2026 and not the period 25 Oct – 24 Nov 2026

#### Scenario: The 25th of November is already the next period
- **WHEN** the user records a 900000 expense in `Food` dated 2026-11-02
- **THEN** it counts toward the period 25 Oct – 24 Nov 2026

### Requirement: Spending categories compare against a limit
For a category in the `needs` or `wants` bucket, the system SHALL show remaining amount as the planned amount minus actual spending, and SHALL show progress as the share of the planned amount already spent. It SHALL round the remaining amount it displays to two decimal places.

#### Scenario: Within the limit
- **WHEN** `Food` is planned at 1500000 and 900000 has been spent in the period
- **THEN** `Food` shows remaining 600000 and progress of 60%

#### Scenario: Overspent
- **WHEN** `Food` is planned at 500000 and 900000 has been spent in the period
- **THEN** `Food` shows a remaining amount of -400000 and is marked as overspent

#### Scenario: A fraction over the limit is overspent
- **WHEN** `Food` is planned at 500000 and 500000.60 has been spent in the period
- **THEN** `Food` is marked as overspent and shows a remaining amount of -0.60

#### Scenario: Spending with no plan
- **WHEN** 900000 has been spent in `Fun` in a period with no planned amount for it
- **THEN** `Fun` shows actual spending of 900000 and no remaining amount or progress

#### Scenario: Plan with no spending
- **WHEN** `Food` is planned at 1500000 and nothing has been spent in the period
- **THEN** `Food` shows remaining 1500000 and progress of 0%

### Requirement: Savings categories compare against a target
For a category in the `savings` bucket, the system SHALL measure actual progress from transfers into that category dated within the period, including transfers flagged as investments.

#### Scenario: Transfer counts toward a savings target
- **WHEN** the user records a 500000 transfer in category `Emergency` dated 2026-10-28
- **THEN** actual progress for `Emergency` in the period 25 Oct – 24 Nov 2026 is 500000

#### Scenario: Investment transfer counts toward a savings target
- **WHEN** the user records a 2000000 transfer in category `Emergency` flagged as an investment, dated 2026-10-28
- **THEN** actual progress for `Emergency` in that period is 2000000

#### Scenario: Expenses do not count toward a savings target
- **WHEN** the user records a 900000 expense in category `Emergency` in the period
- **THEN** actual progress for `Emergency` in that period is unchanged

#### Scenario: Savings target met
- **WHEN** `Emergency` is planned at 1000000 and 1000000 has been transferred into it in the period
- **THEN** `Emergency` shows progress of 100% and no remaining amount

### Requirement: Move between periods
The system SHALL let the user view a previous or next financial period and its planned amounts, actual spending, remaining amounts, and progress.

#### Scenario: Open the previous period
- **WHEN** the user is viewing the period 25 Oct – 24 Nov 2026 and opens the previous period
- **THEN** the system shows the period 25 Sep – 24 Oct 2026 with its own planned amounts and actual spending

#### Scenario: Return to the current period
- **WHEN** the user is viewing an earlier period and opens the current period
- **THEN** the system shows the period that contains today
