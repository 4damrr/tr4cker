# To-Buy List Specification

## Purpose

Lets the user keep a list of things they intend to buy, with enough detail to judge the commitment and to act on it later without affecting the transaction history.

## Requirements

### Requirement: Record an intended purchase
The system SHALL let the user add a to-buy item with a name, a category, a priority, a quantity, an expected price per item, and an optional product URL and notes.

#### Scenario: Add an item
- **WHEN** the user adds item `Keyboard`, category `Electronics`, priority 1, quantity 1, expected price 750000, and no URL
- **THEN** the item appears in the list with those values and a status of still to buy

#### Scenario: Whole quantity and price
- **WHEN** the user adds an item with quantity 2 and expected price 125000
- **THEN** the quantity is stored as 2 and the price as 125000

#### Scenario: Fractional expected price
- **WHEN** the user adds an item with quantity 1 and expected price 74999.99
- **THEN** the price is stored as 74999.99 and the expected total is 74999.99

#### Scenario: Round finer input
- **WHEN** the user submits an expected price of 74999.995
- **THEN** the item is added with a price of 75000.00

#### Scenario: A name and category are required
- **WHEN** the user submits a new item without a name or without a category
- **THEN** the item is not added and the user is told which field is missing

### Requirement: Show the expected total for an item
The system SHALL show an item's expected total cost as its quantity multiplied by its expected price.

#### Scenario: Total for multiple units
- **WHEN** an item has quantity 3 and expected price 125000
- **THEN** the item shows an expected total of 375000

#### Scenario: Total from a fractional price
- **WHEN** an item has quantity 3 and expected price 125000.55
- **THEN** the item shows an expected total of 375001.65

### Requirement: Priorities are ordered highest first
The system SHALL accept only priorities 1, 2, or 3, where 1 is the highest, and SHALL order the list so higher priority items come first within the same status.

#### Scenario: Highest priority first
- **WHEN** the list holds an item with priority 2 and another with priority 1, both still to buy
- **THEN** the priority 1 item is listed above the priority 2 item

#### Scenario: Invalid priority rejected
- **WHEN** the user submits an item with priority 4
- **THEN** the item is not added and the priority is reported as invalid

### Requirement: Track buying status
The system SHALL let the user change an item's status between still to buy, bought, and canceled, and SHALL let the user return a bought or canceled item to still to buy.

#### Scenario: Mark an item bought
- **WHEN** the user marks item `Keyboard` as bought
- **THEN** the item shows a status of bought

#### Scenario: Cancel an item
- **WHEN** the user cancels item `Monitor`
- **THEN** the item shows a status of canceled

#### Scenario: Reopen an item
- **WHEN** the user changes a canceled item back to still to buy
- **THEN** the item shows a status of still to buy

### Requirement: Marking an item bought does not record a transaction
The system MUST NOT create, modify, or delete any transaction or account balance when the user changes a to-buy item's status.

#### Scenario: Bought item leaves balances untouched
- **WHEN** the user marks an item with an expected price of 750000 as bought
- **THEN** no transaction is created and every account balance is unchanged

#### Scenario: Spending is not affected
- **WHEN** the user marks an item bought in category `Electronics`
- **THEN** actual spending for that category in the current period is unchanged

### Requirement: Edit and remove an item
The system SHALL let the user change the name, category, priority, quantity, expected price, URL, and notes of an existing item, and SHALL let the user remove an item.

#### Scenario: Change the expected price
- **WHEN** the user changes an item's expected price from 750000 to 700000
- **THEN** later views show 700000 and the expected total is recalculated

#### Scenario: Remove an item
- **WHEN** the user removes an item from the list
- **THEN** it no longer appears in the list

### Requirement: Filter the list
The system SHALL let the user narrow the list to items with a chosen status, and SHALL show only items that have not been removed.

#### Scenario: Show only items still to buy
- **WHEN** the user filters the list to still to buy
- **THEN** bought and canceled items are not shown

#### Scenario: Default view excludes removed items
- **WHEN** the user opens the to-buy list
- **THEN** every item they have not removed is shown
