## Overview

The application should have a lightweight, minimal interface focused on helping the user understand their current financial situation and quickly record transactions.

The frontend uses:

- HTML rendered by Go
    
- HTMX for dynamic interactions
    
- Minimal JavaScript
    
- No frontend SPA framework
    

The interface should prioritize usability and fast interaction over visual complexity.

## Design Principles

- Minimal and lightweight
    
- Mobile-friendly
    
- Fast transaction entry
    
- Clear financial information
    
- Minimal navigation
    
- Avoid unnecessary animations and UI elements
    
- Important information should be visible without requiring many interactions
    

## Main Pages

The application should initially have the following main areas:

### Dashboard

The dashboard is the main screen of the application.

It should provide a quick overview of the user's current financial situation.

It should show:

- Current financial period
    
- Current balance
    
- Income
    
- Spending
    
- Remaining planned amount
    
- Planning progress
    
- Recent transactions
    

The dashboard should emphasize **progress against the user's plans**, rather than only showing historical transactions.

### Transactions

The transactions page displays the user's transaction history.

It should allow the user to:

- View transactions
    
- Filter transactions
    
- Add a transaction
    
- Edit a transaction
    
- Delete a transaction
    

Transactions should clearly show:

- Date/time
    
- Amount
    
- Type
    
- Category
    
- Account
    

### Add Transaction

Adding a transaction should require as little interaction as possible.

The form should allow the user to enter:

- Transaction type
    
- Amount
    
- Category
    
- Source account
    
- Destination account
    
- Transaction date/time
    
- Optional notes
    

The form should provide sensible defaults where possible.

### Planning

The planning page allows the user to define planned spending for each category.

It should show:

- Current month
    
- Categories
    
- Planned amount
    
- Actual spending
    
- Remaining amount
    
- Progress toward the plan
    

The user should be able to create and modify planning entries.

### Accounts

The accounts page displays the user's accounts.

It should show:

- Account name
    
- Current balance
    
- Notes
    

The user should be able to create, edit, and deactivate accounts.

### To Buy

The to-buy page displays items that the user intends to purchase.

It should show:

- Item name
    
- Category
    
- Priority
    
- Quantity
    
- Expected price
    
- Status
    

The user should be able to add, edit, mark as bought, and cancel items.

## Navigation

The application should have simple navigation between the main areas.

The exact navigation layout is still to be determined.

Possible primary navigation:

- Dashboard
    
- Transactions
    
- Planning
    
- To Buy
    

Account management may be accessible from the dashboard or a secondary navigation area.

## Transaction Entry

Transaction entry is one of the most important interactions in the application.

The user should be able to record a transaction quickly without navigating through multiple pages.

The interface should minimize the number of required fields and interactions.

For example, commonly used categories and accounts should be easy to select.

## Financial Progress

The application should make financial progress easy to understand.

For a planning category, the interface should communicate something similar to:

```text
Food

Planned     Rp 1,500,000
Spent       Rp   900,000
Remaining   Rp   600,000
```

The exact visualization is still to be determined.

## HTMX

HTMX should be used for interactions that benefit from partial page updates.

Examples include:

- Adding a transaction
    
- Editing a transaction
    
- Deleting a transaction
    
- Filtering transactions
    
- Updating planning values
    
- Changing transaction status
    
- Updating dashboard information
    

The application should prefer server-rendered HTML over client-side state management.

## Responsive Design

The application should work well on both desktop and mobile screens.

Mobile usability is particularly important because transaction entry may happen immediately after making a purchase.

The interface should not require a desktop-sized screen for normal daily usage.

## Visual Design

The visual design should be minimal and practical.

Specific choices for:

- Colors
    
- Typography
    
- Spacing
    
- Icons
    
- Components
    
- Navigation layout
    
- Charts
    
- Progress indicators
    

will be defined separately.

## Open Questions

- What should the primary navigation look like?
    
- Should the dashboard be the default page?
    
- Should transaction entry be a modal, drawer, or dedicated page?
    
- Should the application use charts or primarily numbers/progress indicators?
    
- How should the 25th–24th financial period be represented visually?
    
- Should the dashboard show all accounts individually or only the combined balance?
    
- How much information should be shown on mobile versus desktop?