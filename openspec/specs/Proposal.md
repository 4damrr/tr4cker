## Overview

I want to build a lightweight personal money tracker to make me more mindful about my spending and how I use my money in relation to my broader life plans.

The main problem with the money-tracking applications I currently use is that they mostly show historical spending. They do not give me a clear, real-time view of how I am progressing toward the plans and spending targets I set for each financial period.

The goal of this application is to make that progress visible and actionable while keeping the application simple enough that recording a transaction takes very little effort.

## Existing Project

A boilerplate application has already been initialized. The implementation should build upon the existing codebase rather than starting the application from scratch.

Before making architectural or structural changes, inspect the existing project and understand its current structure.

## Technology Constraints

The application should remain lightweight and use the following stack:

- **Backend:** Go
- **Frontend:** HTMX, with HTML generated/rendered by the Go backend
- **Database:** SQLite

Avoid introducing additional frameworks, services, or infrastructure unless they are necessary and justified.

## Design Principles

The application should prioritize:

- Simplicity
- Fast transaction entry
- Clear visibility of current financial progress
- Minimal infrastructure
- A lightweight user experience
- Maintainable and straightforward code

The application should focus on helping the user understand their current financial situation rather than becoming a complex financial-management platform.

## Scope

The initial implementation should establish the foundation for:

- Recording income and expenses
- Categorizing transactions
- Tracking financial periods
- Tracking spending against plans or targets
- Showing current financial progress in real time
- Providing a simple dashboard for understanding the current financial situation

The exact database schema and frontend design will be specified separately and should be treated as user-defined design constraints rather than being invented by the implementation process.