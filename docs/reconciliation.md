# Reconciliation

## Overview

`go-hermes` includes an admin reconciliation report that checks whether the persisted wallet state still agrees with the ledger and transaction history.

The goal is to make balance and ledger corruption visible from persisted state, not to claim a mathematically complete proof of system correctness.

## Admin Endpoint

- `GET /api/v1/admin/reconciliation`

This endpoint is admin-only and returns:

- whether the system is currently healthy
- summary counts for wallets, transactions, and ledger entries scanned
- wallet-level balance mismatches
- transaction-level ledger shape violations
- ledger-level orphan or arithmetic issues

Every reconciliation run is also audit logged.

## Invariants Verified

The reconciliation report currently verifies:

- `wallet.balance` matches the balance derived from ledger credits minus debits for that wallet
- the first ledger entry for a wallet starts from the expected wallet genesis balance of `0`
- ledger entry arithmetic is internally valid:
  - `CREDIT`: `balance_after - balance_before == amount`
  - `DEBIT`: `balance_before - balance_after == amount`
- ledger balance chains are continuous per wallet:
  - the first entry must start from the wallet genesis balance
  - each next `balance_before` must match the previous `balance_after`
- every ledger entry points to an existing wallet
- every ledger entry points to an existing transaction
- `TOP_UP` transactions:
  - have no source wallet
  - have a destination wallet
  - have exactly one ledger entry
  - use a `CREDIT` entry on the destination wallet
  - use a ledger amount equal to the transaction amount
- `TRANSFER` transactions:
  - have both source and destination wallets
  - do not transfer to the same wallet
  - have exactly two ledger entries
  - contain one source-wallet `DEBIT`
  - contain one destination-wallet `CREDIT`
  - use ledger amounts equal to the transaction amount

## What It Detects

Examples of problems the report will surface:

- wallet balance drift
- orphan ledger entries
- missing transfer leg entries
- top up entries posted to the wrong wallet
- mismatched ledger and transaction amounts
- broken ledger balance chains
- impossible first-entry states such as a wallet appearing to start from a non-zero balance without prior ledger history

## Assumptions

The current reconciliation logic depends on a few explicit assumptions about this codebase:

- wallets are created with balance `0`
- every balance mutation is represented by append-only ledger entries
- there is no separate opening-balance or migration-time ledger bootstrap flow

If the project later introduces opening balances or backfilled ledger history, reconciliation would need to evolve with that model.

## What Is Actually Tested

- in-memory tests cover healthy and corrupted reconciliation scenarios quickly
- PostgreSQL-backed tests cover realistic corrupted persisted state, including wallet drift, transaction/ledger mismatches, missing transfer legs, orphan ledger entries, and invalid first ledger entries
- some PostgreSQL corruption tests intentionally bypass constraints inside the test harness so the checker can be validated against impossible-but-persisted states

## Why This Matters

For a wallet or fintech backend, storing transactions is not enough. Senior-level correctness requires the ability to explain and verify:

- what happened
- how balances changed
- whether the current persisted state is still trustworthy

This reconciliation report is a practical detection layer for persisted-state inconsistencies. It materially improves operational trust, but it is still a checker with explicit assumptions rather than a complete proof system.
