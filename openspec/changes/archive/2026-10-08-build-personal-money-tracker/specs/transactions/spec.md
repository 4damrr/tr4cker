# Spec Delta

## Purpose

Lets the user record income, expense, and transfer movements quickly and review that history without treating transfers as spending.

## ADDED Requirements

### Requirement: Record income, expense, and transfer
The system SHALL record a transaction with amount, type (`income`, `expense`, or `transfer`), category, occurrence time, investment flag, source account, and destination account.

#### Scenario: Expense leaves a held account
- **WHEN** the user records type `expense`, amount 25000, category `Food`, source `BRI`, and destination `external`
- **THEN** the system stores an expense of 25000 from `BRI` to `external` and does not treat it as a transfer

#### Scenario: Income enters a held account
- **WHEN** the user records type `income`, amount 5000000, source `external`, and destination `BRI`
- **THEN** the system stores an income of 5000000 to `BRI`

#### Scenario: Transfer between held accounts
- **WHEN** the user records type `transfer`, amount 100000, source `BRI`, and destination `Gopay`
- **THEN** the system stores a transfer from `BRI` to `Gopay` and does not include that amount in spending totals

### Requirement: Transfers are excluded from spending
The system MUST NOT count transfer amounts as spending when calculating spending totals, remaining planned amounts, or planning progress.

#### Scenario: Transfer does not increase spending
- **WHEN** spending for the current period is 900000 and the user records a 100000 transfer between two held accounts
- **THEN** period spending remains 900000

### Requirement: Transaction amounts are floating-point money
The system SHALL store transaction amounts as floating-point values rounded to two decimal places.

#### Scenario: Store a whole amount
- **WHEN** the user records an amount of Rp 25,000
- **THEN** the stored amount is 25000

#### Scenario: Store a fractional amount
- **WHEN** the user records an amount of Rp 25,000.50
- **THEN** the stored amount is 25000.50

#### Scenario: Round finer input
- **WHEN** the user records an amount of Rp 25,000.505
- **THEN** the stored amount is 25000.51

### Requirement: Fast transaction entry
The system SHALL let the user record a transaction with type, amount, category, source account, destination account, and occurrence time, using defaults so common expenses need few inputs.

#### Scenario: Default expense now
- **WHEN** the user opens transaction entry without changing defaults
- **THEN** type is `expense` and occurrence time is the current time

#### Scenario: Required fields for a valid save
- **WHEN** the user submits entry without an amount or without accounts required for the selected type
- **THEN** the system rejects the save and does not create a transaction

### Requirement: List, filter, and edit transactions
The system SHALL show active transactions with date/time, amount, type, category, and accounts, and SHALL allow filtering, editing, and viewing that list.

#### Scenario: View history
- **WHEN** the user opens the transactions area
- **THEN** active transactions are listed with date/time, amount, type, category, and accounts

#### Scenario: Filter by type
- **WHEN** the user filters the list to `expense`
- **THEN** only active expense transactions are shown

#### Scenario: Edit amount
- **WHEN** the user changes an active expense amount from 25000 to 27000 and saves
- **THEN** later views show 27000 for that transaction

### Requirement: Soft-delete transactions
The system SHALL soft-delete a transaction by setting a deletion timestamp, exclude it from active lists and spending calculations, and MUST NOT physically delete the row.

#### Scenario: Remove from active list
- **WHEN** the user deletes an active transaction
- **THEN** it no longer appears in the active transactions list and its amount is excluded from active spending totals
