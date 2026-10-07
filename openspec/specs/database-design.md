## Overview

The application uses SQLite as its database.

The database is designed around five main concepts:

- Transactions    
- Categories
- Accounts
- Monthly planning
- Items to buy

The database should remain lightweight and should not introduce additional entities unless they are necessary for a clearly defined business requirement.

## Transactions

The `transactions` table records movement of money between accounts.

```text
transactions
├── id
├── amount
├── type
├── category
├── trx_time
├── is_investment
├── account_from
├── account_to
├── created_at
└── deleted_at
```

### Fields

| Field           | Type      | Description                        |
| --------------- | --------- | ---------------------------------- |
| `id`            | bigint    | Primary key                        |
| `amount`        | money     | Transaction amount                 |
| `type`          | trx_type  | Type of transaction                |
| `category`      | string    | Category of the transaction        |
| `trx_time`      | timestamp | Time when the transaction occurred |
| `is_investment` | bool      | Is it investment or no             |
| `account_from`  | string    | Account money comes from           |
| `account_to`    | string    | Account money goes to              |
| `created_at`    | timestamp | Record creation time               |
| `deleted_at`    | timestamp | Soft deletion timestamp            |

### Transaction types

```text
income
expense
transfer
```

#### Income

Money enters one of the user's accounts.

Example:

```text
account_from = external
account_to   = BRI
type         = income
```

#### Expense

Money leaves one of the user's accounts.

Example:

```text
account_from = BRI
account_to   = external
type         = expense
```

#### Transfer

Money moves between two accounts owned by the user.

Example:

```text
account_from = BRI
account_to   = Gopay
type         = transfer
```

Transfers should not be counted as spending.

## Categories

The `category` table classifies transactions and planning entries.

```text
category
├── key
├── buckets
└── deleted_at
```

|Field|Type|Description|
|---|---|---|
|`key`|string|Primary key and category identifier|
|`buckets`|buckets|High-level classification|
|`deleted_at`|timestamp|Soft deletion timestamp|

### Buckets

```text
needs
wants
savings
```

Examples:

```text
Food          → needs
Entertainment → wants
Emergency     → savings
```

Categories are shared by transactions and monthly planning.

## Accounts

The `account` table represents places where the user's money is held.

```text
account
├── key
├── balance
├── notes
└── deleted_at
```

| Field        | Type      | Description                        |
| ------------ | --------- | ---------------------------------- |
| `key`        | string    | Primary key and account identifier |
| `balance`    | money     | Current account balance            |
| `notes`      | string    | Optional notes                     |
| `deleted_at` | timestamp | Soft deletion timestamp            |

Examples of accounts:

```text
BRI
Cash
Gopay
ShopeePay
```

The account balance represents the current amount of money available in that account.

Transactions are responsible for recording changes to account balances.

## Monthly Planning

The `planning` table stores the user's planned amount for a category during a specific month.

```text
planning
├── month
├── year
├── category
├── amount
└── notes
```

|Field|Type|Description|
|---|---|---|
|`month`|int|Calendar month|
|`year`|int|Calendar year|
|`category`|string|Category being planned|
|`amount`|money|Planned amount|
|`notes`|string|Optional notes|

The combination of:

```text
month + year + category
```

must be unique.

This means a category can only have one planning entry for a given month.

For example:

```text
2026-10 + Food       → 1,500,000
2026-10 + Transport  →   500,000
2026-10 + Fun        →   300,000
```

Planning is used to compare **planned spending against actual spending**.

## Items to Buy

The `to_buy` table tracks things the user intends to purchase.

```text
to_buy
├── id
├── category
├── name
├── priority
├── qty
├── price
├── url
├── notes
└── status
```

|Field|Type|Description|
|---|---|---|
|`id`|bigint|Primary key|
|`category`|string|Category of the item|
|`name`|string|Item name|
|`priority`|priority|Purchase priority|
|`qty`|int|Quantity|
|`price`|money|Expected price per item|
|`url`|string|Optional product URL|
|`notes`|string|Optional notes|
|`status`|buying_status|Current purchase status|

### Buying status

```text
to_buy
bought
canceled
```

### Priority

```text
1
2
3
```

Where:

- `1` = highest priority
- `2` = medium priority
- `3` = lowest priority
    

## Relationships

```text
category
   │
   ├─────────────── transactions
   │
   ├─────────────── planning
   │
   └─────────────── to_buy

account
   │
   ├─────────────── transactions.account_from
   │
   └─────────────── transactions.account_to
```

## Money Representation

Monetary values should not use floating-point representation.

Although the initial conceptual schema uses `float`, the implementation should use an exact representation suitable for IDR amounts.

For SQLite, the preferred representation is an integer containing the smallest relevant currency unit.

For IDR, this can simply be the integer amount in rupiah.

For example:

```text
Rp 25,000 → 25000
Rp 1,500,000 → 1500000
```

This avoids floating-point rounding problems.

## Soft Deletion

The following tables support soft deletion:

- `transactions`
    
- `category`
    
- `account`
    

A record with a non-null `deleted_at` should normally be excluded from active application queries.

Historical transactions should not be physically deleted because they are part of the user's financial history.

## Business Rules

### Transfers

A transfer between two user accounts must not be counted as an expense.

For example:

```text
BRI → Gopay
```

changes the balances of the two accounts but does not increase spending.

### Expenses

An expense decreases the balance of `account_from`.

The `account_to` represents the destination outside the user's tracked accounts.

### Income

An income increases the balance of `account_to`.

The `account_from` represents the source outside the user's tracked accounts.

### Planning

Planning represents the user's intended spending for a category during a calendar month.

Actual spending should be calculated from transactions and compared against the corresponding planning amount.

## Open Questions

These decisions still need to be finalized:
- Whether `account.balance` is stored as the source of truth or calculated from transactions. (yes)
- Whether account balances should be updated transactionally whenever a transaction is created, modified, or deleted. (yes)
- Whether `planning` follows calendar months or the application's 25th–24th financial period. (application's financial period)
- Whether `planning.amount` represents a spending limit, target, or expected spending.
- Whether `to_buy.price` represents the price of one item or the total expected purchase cost. (one item)
- Whether buying an item should automatically create a transaction. (still need to update it manually)
- Whether `to_buy` items should have their own category separate from financial transaction categories. (no)
- Whether an `investment` transaction should exist separately from `transfer`, or whether investments should be represented as transfers to an investment account. (no)