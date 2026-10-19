# Spec Delta

## Purpose

Lets the user maintain shared spending categories in needs, wants, or savings buckets for transactions, monthly planning, and to-buy items.

## ADDED Requirements

### Requirement: Categories have a key and bucket
The system SHALL identify each category by a unique key and SHALL assign it to exactly one bucket: `needs`, `wants`, or `savings`.

#### Scenario: Create a needs category
- **WHEN** the user creates category key `Food` with bucket `needs`
- **THEN** `Food` is available as a needs category for transactions, planning, and to-buy items

#### Scenario: Create a wants category
- **WHEN** the user creates category key `Entertainment` with bucket `wants`
- **THEN** `Entertainment` is stored as a wants category

#### Scenario: Create a savings category
- **WHEN** the user creates category key `Emergency` with bucket `savings`
- **THEN** `Emergency` is stored as a savings category

### Requirement: Only active categories are offered for new records
The system SHALL offer only non-deleted categories when the user records a new transaction, planning entry, or to-buy item.

#### Scenario: Deleted category hidden from new entry
- **WHEN** category `Fun` has been soft-deleted
- **THEN** `Fun` is not offered as a category choice on new transaction, planning, or to-buy forms

### Requirement: Soft-delete categories
The system SHALL deactivate a category by setting a deletion timestamp and MUST NOT physically delete the category row.

#### Scenario: Deactivate a category
- **WHEN** the user deactivates category `Entertainment`
- **THEN** `Entertainment` is excluded from active category lists and from category choices on new records
