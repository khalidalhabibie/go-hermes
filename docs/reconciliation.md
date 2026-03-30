# Reconciliation

## Overview

`go-hermes` includes an admin reconciliation report that checks whether the persisted wallet state still agrees with the ledger and transaction history.

The goal is not just to record money movement, but to prove that the core financial invariants still hold.

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
- ledger entry arithmetic is internally valid:
  - `CREDIT`: `balance_after - balance_before == amount`
  - `DEBIT`: `balance_before - balance_after == amount`
- ledger balance chains are continuous per wallet:
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

## Why This Matters

For a wallet or fintech backend, storing transactions is not enough. Senior-level correctness requires the ability to explain and verify:

- what happened
- how balances changed
- whether the current persisted state is still trustworthy

This reconciliation report is the first operational proof layer for that.
