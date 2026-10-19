# Accounts Specification

## Purpose

Lets the user manage the accounts where money is held, and keeps each account's stored balance consistent with the transactions recorded against it.

## Requirements

### Requirement: Accounts are identified by a key
The system SHALL identify each account by a unique key, SHALL store its balance as a floating-point value, and SHALL allow optional notes.

#### Scenario: Create an account
- **WHEN** the user creates account `BRI` with a starting balance of 2500000 and no notes
- **THEN** `BRI` is listed with balance 2500000 and no notes

#### Scenario: A fractional starting balance is kept
- **WHEN** the user creates an account with a starting balance of 25000.60
- **THEN** the stored balance is 25000.60

#### Scenario: Finer input is rounded
- **WHEN** the user creates an account with a starting balance of 25000.605
- **THEN** the stored balance is 25000.61

### Requirement: Edit an account's balance and notes
The system SHALL let the user change the balance and notes of an active account. The key SHALL stay fixed after creation, and recorded transactions SHALL keep the account key they were created with.

#### Scenario: Correct a balance
- **WHEN** the user changes the `BRI` balance from 2500000 to 2600000 and saves
- **THEN** later views show 2600000 for `BRI`

#### Scenario: Add notes
- **WHEN** the user adds notes `primary checking` to account `BRI`
- **THEN** the account list shows those notes next to `BRI`

### Requirement: Offer only active accounts on new records
The system SHALL offer only accounts that have not been deactivated when the user records a transaction.

#### Scenario: Deactivated account hidden from new entry
- **WHEN** account `Gopay` has been deactivated
- **THEN** `Gopay` is not offered as a source or destination account on a new transaction

### Requirement: Deactivate accounts without losing history
The system SHALL deactivate an account by setting a deletion timestamp, MUST NOT physically delete the account row, and MUST preserve its stored balance and its recorded history.

#### Scenario: Deactivate an account
- **WHEN** the user deactivates account `Gopay`
- **THEN** `Gopay` is excluded from active account lists and from account choices on new records

#### Scenario: History survives deactivation
- **WHEN** the user views past transactions recorded against `Gopay` after deactivating it
- **THEN** those transactions still show `Gopay` and its balance still appears in totals

### Requirement: Balances are stored rounded to two decimal places
The system SHALL store every account balance rounded to two decimal places, and SHALL apply that rounding whenever a transaction changes a balance.

#### Scenario: Balance keeps its cents after a fractional expense
- **WHEN** the balance is 100000 and the user records a 25000.50 expense
- **THEN** the stored balance is 74999.50

#### Scenario: Rounding applies to the resulting balance
- **WHEN** the balance is 0 and the user records a 0.756 expense
- **THEN** the stored balance is -0.76

### Requirement: Balances follow transaction activity
The system SHALL adjust the stored balance of the accounts a transaction touches when that transaction is created, edited, or soft-deleted.

#### Scenario: Expense reduces the source balance
- **WHEN** the user records an expense of 25000 from `BRI` to `external`
- **THEN** the `BRI` balance is reduced by 25000

#### Scenario: Income increases the destination balance
- **WHEN** the user records an income of 5000000 from `external` to `BRI`
- **THEN** the `BRI` balance is increased by 5000000

#### Scenario: Transfer moves the amount
- **WHEN** the user records a transfer of 100000 from `BRI` to `Gopay`
- **THEN** the `BRI` balance is reduced by 100000 and the `Gopay` balance is increased by 100000

#### Scenario: Editing an amount moves the difference
- **WHEN** the user changes an active expense from 25000 to 27000
- **THEN** the source account balance reflects a decrease of 27000 for that transaction

#### Scenario: Soft-deleting restores the balance
- **WHEN** the user soft-deletes an active expense of 25000 from `BRI`
- **THEN** the `BRI` balance is increased by 25000 and the transaction is excluded from active lists

### Requirement: A balance change commits with its transaction
The system MUST commit a balance change together with the transaction that caused it, and MUST leave both unchanged if the transaction cannot be saved.

#### Scenario: Invalid transaction leaves balances untouched
- **WHEN** a transaction save fails validation after it would have adjusted two account balances
- **THEN** neither account balance changes and no transaction is recorded
