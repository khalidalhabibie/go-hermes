package usecase

import (
	"context"
	"encoding/json"
	"time"

	"go-hermes/internal/entity"
	"go-hermes/internal/pkg/apiresponse"
	"go-hermes/internal/pkg/apperror"
	"go-hermes/internal/pkg/pagination"
	"go-hermes/internal/repository"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const (
	topUpEndpoint    = "POST /api/v1/wallets/me/top-up"
	transferEndpoint = "POST /api/v1/transfers"
)

type TransactionUsecase struct {
	txManager    repository.TransactionManager
	wallets      repository.WalletRepository
	transactions repository.TransactionRepository
	ledgers      repository.LedgerRepository
	idempotency  repository.IdempotencyRepository
	audits       repository.AuditLogRepository
	webhooks     WebhookNotifier
}

func NewTransactionUsecase(
	txManager repository.TransactionManager,
	wallets repository.WalletRepository,
	transactions repository.TransactionRepository,
	ledgers repository.LedgerRepository,
	idempotency repository.IdempotencyRepository,
	audits repository.AuditLogRepository,
	webhooks WebhookNotifier,
) *TransactionUsecase {
	if webhooks == nil {
		webhooks = &NoopWebhookService{}
	}
	return &TransactionUsecase{
		txManager:    txManager,
		wallets:      wallets,
		transactions: transactions,
		ledgers:      ledgers,
		idempotency:  idempotency,
		audits:       audits,
		webhooks:     webhooks,
	}
}

func (u *TransactionUsecase) TopUp(ctx context.Context, req TopUpRequest) (*OperationResult, error) {
	if req.IdempotencyKey == "" {
		return nil, apperror.Validation([]map[string]string{{"field": "Idempotency-Key", "message": "header is required"}})
	}
	if req.Amount <= 0 {
		return nil, apperror.Validation([]map[string]string{{"field": "amount", "message": "must be greater than zero"}})
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return nil, apperror.BadRequest("invalid user id")
	}

	var result OperationResult
	var deliveryToQueue *uuid.UUID
	err = u.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		record := &entity.IdempotencyRecord{
			ID:             uuid.New(),
			IdempotencyKey: req.IdempotencyKey,
			UserID:         userID,
			RequestHash:    req.RequestHash,
			Endpoint:       topUpEndpoint,
			StatusCode:     0,
			ResponseBody:   datatypes.JSON([]byte("{}")),
		}

		inserted, err := u.idempotency.Reserve(txCtx, record)
		if err != nil {
			return err
		}
		if !inserted {
			existing, err := u.idempotency.Get(txCtx, req.IdempotencyKey, userID, topUpEndpoint)
			if err != nil {
				return err
			}
			if existing.RequestHash != req.RequestHash {
				return apperror.Conflict("idempotency key already used with different payload")
			}
			if existing.StatusCode == 0 {
				return apperror.Conflict("request with the same idempotency key is still being processed")
			}

			result = OperationResult{
				StatusCode: existing.StatusCode,
				Body:       []byte(existing.ResponseBody),
				Replay:     true,
			}
			return nil
		}

		wallet, err := u.wallets.GetByUserID(txCtx, userID)
		if err != nil {
			return err
		}

		lockedWallets, err := u.wallets.LockByIDs(txCtx, []uuid.UUID{wallet.ID})
		if err != nil {
			return err
		}
		if len(lockedWallets) != 1 {
			return apperror.NotFound("wallet not found")
		}

		targetWallet := lockedWallets[0]
		if targetWallet.Status != entity.WalletStatusActive {
			return apperror.Conflict("wallet is not active")
		}

		beforeBalance := targetWallet.Balance
		targetWallet.Balance += req.Amount

		transaction := &entity.Transaction{
			ID:                  uuid.New(),
			TransactionRef:      "TXN-" + uuid.NewString(),
			Type:                entity.TransactionTypeTopUp,
			Status:              entity.TransactionStatusSuccess,
			DestinationWalletID: &targetWallet.ID,
			Amount:              req.Amount,
			Currency:            "IDR",
			IdempotencyKey:      req.IdempotencyKey,
			InitiatedByUserID:   userID,
			Description:         req.Description,
		}
		if err := u.transactions.Create(txCtx, transaction); err != nil {
			return err
		}

		if err := u.wallets.Update(txCtx, &targetWallet); err != nil {
			return err
		}

		entry := entity.LedgerEntry{
			ID:            uuid.New(),
			TransactionID: transaction.ID,
			WalletID:      targetWallet.ID,
			EntryType:     entity.LedgerEntryTypeCredit,
			Amount:        req.Amount,
			BalanceBefore: beforeBalance,
			BalanceAfter:  targetWallet.Balance,
			CreatedAt:     time.Now(),
		}
		if err := u.ledgers.CreateMany(txCtx, []entity.LedgerEntry{entry}); err != nil {
			return err
		}

		metadata, _ := json.Marshal(map[string]interface{}{
			"wallet_id": targetWallet.ID.String(),
			"amount":    req.Amount,
		})
		if err := u.audits.Create(txCtx, &entity.AuditLog{
			ID:          uuid.New(),
			ActorUserID: &userID,
			Action:      "WALLET_TOP_UP",
			EntityType:  "transaction",
			EntityID:    &transaction.ID,
			Metadata:    datatypes.JSON(metadata),
			CreatedAt:   time.Now(),
		}); err != nil {
			return err
		}

		payload := apiresponse.MustMarshalSuccess(map[string]interface{}{
			"transaction": toTransactionResponse(transaction),
			"wallet":      toWalletResponse(&targetWallet),
		}, nil)

		delivery, err := u.webhooks.CreateTopUpDelivery(txCtx, transaction, &targetWallet)
		if err != nil {
			return err
		}
		if delivery != nil {
			deliveryID := delivery.ID
			deliveryToQueue = &deliveryID
		}

		if err := u.idempotency.MarkCompleted(txCtx, record.ID, 200, payload); err != nil {
			return err
		}

		result = OperationResult{
			StatusCode: 200,
			Body:       payload,
		}
		return nil
	})
	if err != nil {
		return nil, apperror.As(err)
	}
	if deliveryToQueue != nil {
		u.webhooks.Enqueue(ctx, *deliveryToQueue)
	}

	return &result, nil
}

func (u *TransactionUsecase) Transfer(ctx context.Context, req TransferRequest) (*OperationResult, error) {
	if req.IdempotencyKey == "" {
		return nil, apperror.Validation([]map[string]string{{"field": "Idempotency-Key", "message": "header is required"}})
	}
	if req.Amount <= 0 {
		return nil, apperror.Validation([]map[string]string{{"field": "amount", "message": "must be greater than zero"}})
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return nil, apperror.BadRequest("invalid user id")
	}
	recipientWalletID, err := uuid.Parse(req.RecipientWalletID)
	if err != nil {
		return nil, apperror.Validation([]map[string]string{{"field": "recipient_wallet_id", "message": "must be a valid UUID"}})
	}

	var result OperationResult
	var deliveryToQueue *uuid.UUID
	err = u.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		record := &entity.IdempotencyRecord{
			ID:             uuid.New(),
			IdempotencyKey: req.IdempotencyKey,
			UserID:         userID,
			RequestHash:    req.RequestHash,
			Endpoint:       transferEndpoint,
			StatusCode:     0,
			ResponseBody:   datatypes.JSON([]byte("{}")),
		}

		inserted, err := u.idempotency.Reserve(txCtx, record)
		if err != nil {
			return err
		}
		if !inserted {
			existing, err := u.idempotency.Get(txCtx, req.IdempotencyKey, userID, transferEndpoint)
			if err != nil {
				return err
			}
			if existing.RequestHash != req.RequestHash {
				return apperror.Conflict("idempotency key already used with different payload")
			}
			if existing.StatusCode == 0 {
				return apperror.Conflict("request with the same idempotency key is still being processed")
			}

			result = OperationResult{
				StatusCode: existing.StatusCode,
				Body:       []byte(existing.ResponseBody),
				Replay:     true,
			}
			return nil
		}

		senderWallet, err := u.wallets.GetByUserID(txCtx, userID)
		if err != nil {
			return err
		}
		if senderWallet.ID == recipientWalletID {
			return apperror.BadRequest("sender and recipient wallet cannot be the same")
		}

		lockedWallets, err := u.wallets.LockByIDs(txCtx, []uuid.UUID{senderWallet.ID, recipientWalletID})
		if err != nil {
			return err
		}
		if len(lockedWallets) != 2 {
			return apperror.NotFound("wallet not found")
		}

		walletMap := make(map[uuid.UUID]*entity.Wallet, len(lockedWallets))
		for i := range lockedWallets {
			walletMap[lockedWallets[i].ID] = &lockedWallets[i]
		}

		lockedSender := walletMap[senderWallet.ID]
		lockedRecipient := walletMap[recipientWalletID]
		if lockedSender == nil || lockedRecipient == nil {
			return apperror.NotFound("wallet not found")
		}
		if lockedSender.Status != entity.WalletStatusActive || lockedRecipient.Status != entity.WalletStatusActive {
			return apperror.Conflict("wallet is not active")
		}
		if lockedSender.Balance < req.Amount {
			return apperror.Conflict("insufficient balance")
		}

		senderBefore := lockedSender.Balance
		recipientBefore := lockedRecipient.Balance
		lockedSender.Balance -= req.Amount
		lockedRecipient.Balance += req.Amount

		transaction := &entity.Transaction{
			ID:                  uuid.New(),
			TransactionRef:      "TXN-" + uuid.NewString(),
			Type:                entity.TransactionTypeTransfer,
			Status:              entity.TransactionStatusSuccess,
			SourceWalletID:      &lockedSender.ID,
			DestinationWalletID: &lockedRecipient.ID,
			Amount:              req.Amount,
			Currency:            "IDR",
			IdempotencyKey:      req.IdempotencyKey,
			InitiatedByUserID:   userID,
			Description:         req.Description,
		}
		if err := u.transactions.Create(txCtx, transaction); err != nil {
			return err
		}

		if err := u.wallets.Update(txCtx, lockedSender); err != nil {
			return err
		}
		if err := u.wallets.Update(txCtx, lockedRecipient); err != nil {
			return err
		}

		entries := []entity.LedgerEntry{
			{
				ID:            uuid.New(),
				TransactionID: transaction.ID,
				WalletID:      lockedSender.ID,
				EntryType:     entity.LedgerEntryTypeDebit,
				Amount:        req.Amount,
				BalanceBefore: senderBefore,
				BalanceAfter:  lockedSender.Balance,
				CreatedAt:     time.Now(),
			},
			{
				ID:            uuid.New(),
				TransactionID: transaction.ID,
				WalletID:      lockedRecipient.ID,
				EntryType:     entity.LedgerEntryTypeCredit,
				Amount:        req.Amount,
				BalanceBefore: recipientBefore,
				BalanceAfter:  lockedRecipient.Balance,
				CreatedAt:     time.Now(),
			},
		}
		if err := u.ledgers.CreateMany(txCtx, entries); err != nil {
			return err
		}

		metadata, _ := json.Marshal(map[string]interface{}{
			"source_wallet_id":      lockedSender.ID.String(),
			"destination_wallet_id": lockedRecipient.ID.String(),
			"amount":                req.Amount,
		})
		if err := u.audits.Create(txCtx, &entity.AuditLog{
			ID:          uuid.New(),
			ActorUserID: &userID,
			Action:      "WALLET_TRANSFER",
			EntityType:  "transaction",
			EntityID:    &transaction.ID,
			Metadata:    datatypes.JSON(metadata),
			CreatedAt:   time.Now(),
		}); err != nil {
			return err
		}

		payload := apiresponse.MustMarshalSuccess(map[string]interface{}{
			"transaction":        toTransactionResponse(transaction),
			"source_wallet":      toWalletResponse(lockedSender),
			"destination_wallet": toWalletResponse(lockedRecipient),
		}, nil)

		delivery, err := u.webhooks.CreateTransferDelivery(txCtx, transaction, lockedSender, lockedRecipient)
		if err != nil {
			return err
		}
		if delivery != nil {
			deliveryID := delivery.ID
			deliveryToQueue = &deliveryID
		}
		if err := u.idempotency.MarkCompleted(txCtx, record.ID, 200, payload); err != nil {
			return err
		}

		result = OperationResult{
			StatusCode: 200,
			Body:       payload,
		}
		return nil
	})
	if err != nil {
		return nil, apperror.As(err)
	}
	if deliveryToQueue != nil {
		u.webhooks.Enqueue(ctx, *deliveryToQueue)
	}

	return &result, nil
}

func (u *TransactionUsecase) ListMyTransactions(ctx context.Context, userID string, params pagination.Params) ([]TransactionResponse, map[string]interface{}, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, nil, apperror.BadRequest("invalid user id")
	}

	transactions, total, err := u.transactions.ListByUser(ctx, parsedUserID, params)
	if err != nil {
		return nil, nil, apperror.Internal(err)
	}

	items := make([]TransactionResponse, 0, len(transactions))
	for i := range transactions {
		items = append(items, toTransactionResponse(&transactions[i]))
	}
	return items, pagination.Meta(total, params), nil
}

func (u *TransactionUsecase) GetMyTransaction(ctx context.Context, userID, transactionID string) (*TransactionResponse, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, apperror.BadRequest("invalid user id")
	}
	parsedTransactionID, err := uuid.Parse(transactionID)
	if err != nil {
		return nil, apperror.BadRequest("invalid transaction id")
	}

	transaction, err := u.transactions.GetByIDForUser(ctx, parsedTransactionID, parsedUserID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, apperror.NotFound("transaction not found")
		}
		return nil, apperror.Internal(err)
	}
	response := toTransactionResponse(transaction)
	return &response, nil
}

func (u *TransactionUsecase) ListMyLedgers(ctx context.Context, userID string, params pagination.Params) ([]LedgerResponse, map[string]interface{}, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, nil, apperror.BadRequest("invalid user id")
	}

	entries, total, err := u.ledgers.ListByUser(ctx, parsedUserID, params)
	if err != nil {
		return nil, nil, apperror.Internal(err)
	}

	items := make([]LedgerResponse, 0, len(entries))
	for i := range entries {
		items = append(items, toLedgerResponse(&entries[i]))
	}
	return items, pagination.Meta(total, params), nil
}

func (u *TransactionUsecase) ListTransactionLedgers(ctx context.Context, userID, transactionID string) ([]LedgerResponse, error) {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, apperror.BadRequest("invalid user id")
	}
	parsedTransactionID, err := uuid.Parse(transactionID)
	if err != nil {
		return nil, apperror.BadRequest("invalid transaction id")
	}

	entries, err := u.ledgers.ListByTransactionForUser(ctx, parsedTransactionID, parsedUserID)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	items := make([]LedgerResponse, 0, len(entries))
	for i := range entries {
		items = append(items, toLedgerResponse(&entries[i]))
	}
	return items, nil
}
