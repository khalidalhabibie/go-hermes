package usecase

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"go-hermes/internal/entity"
	"go-hermes/internal/pkg/apperror"
	"go-hermes/internal/repository"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type ReconciliationUsecase struct {
	reconciliation repository.ReconciliationRepository
	audits         repository.AuditLogRepository
}

func NewReconciliationUsecase(reconciliation repository.ReconciliationRepository, audits repository.AuditLogRepository) *ReconciliationUsecase {
	return &ReconciliationUsecase{
		reconciliation: reconciliation,
		audits:         audits,
	}
}

func (u *ReconciliationUsecase) Run(ctx context.Context, actorUserID string) (*ReconciliationResponse, error) {
	actorID, err := uuid.Parse(actorUserID)
	if err != nil {
		return nil, apperror.BadRequest("invalid user id")
	}

	wallets, err := u.reconciliation.ListWallets(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	transactions, err := u.reconciliation.ListTransactions(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	ledgerEntries, err := u.reconciliation.ListLedgerEntries(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	report := reconcileState(wallets, transactions, ledgerEntries)

	metadata, _ := json.Marshal(map[string]interface{}{
		"wallets_checked":        report.Summary.WalletsChecked,
		"transactions_checked":   report.Summary.TransactionsChecked,
		"ledger_entries_checked": report.Summary.LedgerEntriesChecked,
		"issue_count":            report.Summary.IssueCount,
		"healthy":                report.Healthy,
	})
	if err := u.audits.Create(ctx, &entity.AuditLog{
		ID:          uuid.New(),
		ActorUserID: &actorID,
		Action:      "ADMIN_RUN_RECONCILIATION",
		EntityType:  "reconciliation",
		Metadata:    datatypes.JSON(metadata),
		CreatedAt:   time.Now(),
	}); err != nil {
		return nil, apperror.Internal(err)
	}

	return &report, nil
}

func reconcileState(wallets []entity.Wallet, transactions []entity.Transaction, ledgerEntries []entity.LedgerEntry) ReconciliationResponse {
	report := ReconciliationResponse{
		CheckedAt: time.Now().UTC(),
	}

	walletsByID := make(map[uuid.UUID]entity.Wallet, len(wallets))
	for _, wallet := range wallets {
		walletsByID[wallet.ID] = wallet
	}

	transactionsByID := make(map[uuid.UUID]entity.Transaction, len(transactions))
	for _, transaction := range transactions {
		transactionsByID[transaction.ID] = transaction
	}

	ledgersByWallet := make(map[uuid.UUID][]entity.LedgerEntry)
	ledgersByTransaction := make(map[uuid.UUID][]entity.LedgerEntry)
	for _, entry := range ledgerEntries {
		if _, ok := walletsByID[entry.WalletID]; !ok {
			report.LedgerEntries = append(report.LedgerEntries, ReconciliationLedgerIssueResponse{
				LedgerEntryID: entry.ID.String(),
				TransactionID: entry.TransactionID.String(),
				WalletID:      entry.WalletID.String(),
				Reason:        "ledger entry references a wallet that does not exist",
			})
		} else {
			ledgersByWallet[entry.WalletID] = append(ledgersByWallet[entry.WalletID], entry)
		}

		if _, ok := transactionsByID[entry.TransactionID]; !ok {
			report.LedgerEntries = append(report.LedgerEntries, ReconciliationLedgerIssueResponse{
				LedgerEntryID: entry.ID.String(),
				TransactionID: entry.TransactionID.String(),
				WalletID:      entry.WalletID.String(),
				Reason:        "ledger entry is orphaned from any transaction",
			})
		} else {
			ledgersByTransaction[entry.TransactionID] = append(ledgersByTransaction[entry.TransactionID], entry)
		}

		if reason := validateLedgerEntryMath(entry); reason != "" {
			report.LedgerEntries = append(report.LedgerEntries, ReconciliationLedgerIssueResponse{
				LedgerEntryID: entry.ID.String(),
				TransactionID: entry.TransactionID.String(),
				WalletID:      entry.WalletID.String(),
				Reason:        reason,
			})
		}
	}

	for _, wallet := range wallets {
		entries := append([]entity.LedgerEntry(nil), ledgersByWallet[wallet.ID]...)
		sortLedgerEntries(entries)

		derivedBalance := int64(0)
		for i, entry := range entries {
			switch entry.EntryType {
			case entity.LedgerEntryTypeCredit:
				derivedBalance += entry.Amount
			case entity.LedgerEntryTypeDebit:
				derivedBalance -= entry.Amount
			}

			if i > 0 && entries[i-1].BalanceAfter != entry.BalanceBefore {
				report.LedgerEntries = append(report.LedgerEntries, ReconciliationLedgerIssueResponse{
					LedgerEntryID: entry.ID.String(),
					TransactionID: entry.TransactionID.String(),
					WalletID:      entry.WalletID.String(),
					Reason:        "ledger balance chain is broken for this wallet",
				})
			}
		}

		if derivedBalance != wallet.Balance {
			report.Wallets = append(report.Wallets, ReconciliationWalletIssueResponse{
				WalletID:         wallet.ID.String(),
				UserID:           wallet.UserID.String(),
				StoredBalance:    wallet.Balance,
				DerivedBalance:   derivedBalance,
				LedgerEntryCount: len(entries),
				Reason:           "wallet balance does not match ledger-derived balance",
			})
		}
	}

	for _, transaction := range transactions {
		entries := append([]entity.LedgerEntry(nil), ledgersByTransaction[transaction.ID]...)
		sortLedgerEntries(entries)

		switch transaction.Type {
		case entity.TransactionTypeTopUp:
			report.Transactions = append(report.Transactions, validateTopUpTransaction(transaction, entries)...)
		case entity.TransactionTypeTransfer:
			report.Transactions = append(report.Transactions, validateTransferTransaction(transaction, entries)...)
		default:
			report.Transactions = append(report.Transactions, newTransactionIssue(transaction, entries, "transaction type is not recognized by reconciliation"))
		}
	}

	report.Summary = ReconciliationSummaryResponse{
		WalletsChecked:             len(wallets),
		TransactionsChecked:        len(transactions),
		LedgerEntriesChecked:       len(ledgerEntries),
		WalletBalanceMismatchCount: len(report.Wallets),
		TransactionIssueCount:      len(report.Transactions),
		LedgerIssueCount:           len(report.LedgerEntries),
		IssueCount:                 len(report.Wallets) + len(report.Transactions) + len(report.LedgerEntries),
	}
	report.Healthy = report.Summary.IssueCount == 0

	return report
}

func validateLedgerEntryMath(entry entity.LedgerEntry) string {
	switch entry.EntryType {
	case entity.LedgerEntryTypeCredit:
		if entry.BalanceAfter-entry.BalanceBefore != entry.Amount {
			return "credit ledger entry does not satisfy balance_after - balance_before == amount"
		}
	case entity.LedgerEntryTypeDebit:
		if entry.BalanceBefore-entry.BalanceAfter != entry.Amount {
			return "debit ledger entry does not satisfy balance_before - balance_after == amount"
		}
	default:
		return "ledger entry type is not recognized by reconciliation"
	}

	if entry.Amount <= 0 {
		return "ledger entry amount must be positive"
	}
	return ""
}

func validateTopUpTransaction(transaction entity.Transaction, entries []entity.LedgerEntry) []ReconciliationTransactionIssueResponse {
	issues := make([]ReconciliationTransactionIssueResponse, 0)

	if transaction.SourceWalletID != nil {
		issues = append(issues, newTransactionIssue(transaction, entries, "top up transaction must not have a source wallet"))
	}
	if transaction.DestinationWalletID == nil {
		issues = append(issues, newTransactionIssue(transaction, entries, "top up transaction must have a destination wallet"))
	}
	if len(entries) != 1 {
		issues = append(issues, newTransactionIssue(transaction, entries, "top up transaction must have exactly one ledger entry"))
		return issues
	}

	entry := entries[0]
	if transaction.DestinationWalletID != nil && entry.WalletID != *transaction.DestinationWalletID {
		issues = append(issues, newTransactionIssue(transaction, entries, "top up ledger entry wallet does not match destination wallet"))
	}
	if entry.EntryType != entity.LedgerEntryTypeCredit {
		issues = append(issues, newTransactionIssue(transaction, entries, "top up ledger entry must be a CREDIT"))
	}
	if entry.Amount != transaction.Amount {
		issues = append(issues, newTransactionIssue(transaction, entries, "top up ledger amount does not match transaction amount"))
	}
	return issues
}

func validateTransferTransaction(transaction entity.Transaction, entries []entity.LedgerEntry) []ReconciliationTransactionIssueResponse {
	issues := make([]ReconciliationTransactionIssueResponse, 0)

	if transaction.SourceWalletID == nil {
		issues = append(issues, newTransactionIssue(transaction, entries, "transfer transaction must have a source wallet"))
	}
	if transaction.DestinationWalletID == nil {
		issues = append(issues, newTransactionIssue(transaction, entries, "transfer transaction must have a destination wallet"))
	}
	if transaction.SourceWalletID != nil && transaction.DestinationWalletID != nil && *transaction.SourceWalletID == *transaction.DestinationWalletID {
		issues = append(issues, newTransactionIssue(transaction, entries, "transfer transaction source and destination wallets must differ"))
	}
	if len(entries) != 2 {
		issues = append(issues, newTransactionIssue(transaction, entries, "transfer transaction must have exactly two ledger entries"))
		return issues
	}

	var foundDebit bool
	var foundCredit bool
	for _, entry := range entries {
		if entry.Amount != transaction.Amount {
			issues = append(issues, newTransactionIssue(transaction, entries, "transfer ledger amounts must match transaction amount"))
			break
		}
		if transaction.SourceWalletID != nil && entry.WalletID == *transaction.SourceWalletID && entry.EntryType == entity.LedgerEntryTypeDebit {
			foundDebit = true
		}
		if transaction.DestinationWalletID != nil && entry.WalletID == *transaction.DestinationWalletID && entry.EntryType == entity.LedgerEntryTypeCredit {
			foundCredit = true
		}
	}

	if !foundDebit {
		issues = append(issues, newTransactionIssue(transaction, entries, "transfer transaction is missing the source-wallet DEBIT ledger entry"))
	}
	if !foundCredit {
		issues = append(issues, newTransactionIssue(transaction, entries, "transfer transaction is missing the destination-wallet CREDIT ledger entry"))
	}

	return issues
}

func newTransactionIssue(transaction entity.Transaction, entries []entity.LedgerEntry, reason string) ReconciliationTransactionIssueResponse {
	ledgerEntryIDs := make([]string, 0, len(entries))
	for _, entry := range entries {
		ledgerEntryIDs = append(ledgerEntryIDs, entry.ID.String())
	}

	return ReconciliationTransactionIssueResponse{
		TransactionID:  transaction.ID.String(),
		TransactionRef: transaction.TransactionRef,
		Type:           string(transaction.Type),
		Amount:         transaction.Amount,
		LedgerEntryIDs: ledgerEntryIDs,
		Reason:         reason,
	}
}

func sortLedgerEntries(entries []entity.LedgerEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].CreatedAt.Equal(entries[j].CreatedAt) {
			return entries[i].ID.String() < entries[j].ID.String()
		}
		return entries[i].CreatedAt.Before(entries[j].CreatedAt)
	})
}
