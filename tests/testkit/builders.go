package testkit

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"go-hermes/internal/entity"
	"go-hermes/internal/pkg/hash"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var builderCounter uint64

type UserBuilder struct {
	name     string
	email    string
	password string
	role     entity.Role
}

func NewUserBuilder() *UserBuilder {
	index := atomic.AddUint64(&builderCounter, 1)
	return &UserBuilder{
		name:     fmt.Sprintf("User %d", index),
		email:    fmt.Sprintf("user%d@example.com", index),
		password: "Password123",
		role:     entity.RoleUser,
	}
}

func (b *UserBuilder) WithName(name string) *UserBuilder {
	b.name = name
	return b
}

func (b *UserBuilder) WithEmail(email string) *UserBuilder {
	b.email = email
	return b
}

func (b *UserBuilder) WithPassword(password string) *UserBuilder {
	b.password = password
	return b
}

func (b *UserBuilder) WithRole(role entity.Role) *UserBuilder {
	b.role = role
	return b
}

func (b *UserBuilder) Build(t *testing.T) (*entity.User, string) {
	t.Helper()

	passwordHash, err := hash.Password(b.password)
	require.NoError(t, err)

	now := time.Now()
	return &entity.User{
		ID:           uuid.New(),
		Name:         b.name,
		Email:        b.email,
		PasswordHash: passwordHash,
		Role:         b.role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, b.password
}

type WalletBuilder struct {
	userID  uuid.UUID
	balance int64
	status  entity.WalletStatus
}

func NewWalletBuilder() *WalletBuilder {
	return &WalletBuilder{
		userID:  uuid.New(),
		balance: 0,
		status:  entity.WalletStatusActive,
	}
}

func (b *WalletBuilder) WithUserID(userID uuid.UUID) *WalletBuilder {
	b.userID = userID
	return b
}

func (b *WalletBuilder) WithBalance(balance int64) *WalletBuilder {
	b.balance = balance
	return b
}

func (b *WalletBuilder) WithStatus(status entity.WalletStatus) *WalletBuilder {
	b.status = status
	return b
}

func (b *WalletBuilder) Build() *entity.Wallet {
	now := time.Now()
	return &entity.Wallet{
		ID:        uuid.New(),
		UserID:    b.userID,
		Balance:   b.balance,
		Status:    b.status,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

type TransactionBuilder struct {
	transactionType entity.TransactionType
	status          entity.TransactionStatus
	sourceWalletID  *uuid.UUID
	destWalletID    *uuid.UUID
	amount          int64
	userID          uuid.UUID
	description     *string
	idempotencyKey  string
}

func NewTransactionBuilder() *TransactionBuilder {
	return &TransactionBuilder{
		transactionType: entity.TransactionTypeTopUp,
		status:          entity.TransactionStatusSuccess,
		amount:          1000,
		userID:          uuid.New(),
		idempotencyKey:  "test-idempotency-key",
	}
}

func (b *TransactionBuilder) WithType(transactionType entity.TransactionType) *TransactionBuilder {
	b.transactionType = transactionType
	return b
}

func (b *TransactionBuilder) WithSourceWalletID(walletID uuid.UUID) *TransactionBuilder {
	b.sourceWalletID = &walletID
	return b
}

func (b *TransactionBuilder) WithDestinationWalletID(walletID uuid.UUID) *TransactionBuilder {
	b.destWalletID = &walletID
	return b
}

func (b *TransactionBuilder) WithAmount(amount int64) *TransactionBuilder {
	b.amount = amount
	return b
}

func (b *TransactionBuilder) WithInitiatedByUserID(userID uuid.UUID) *TransactionBuilder {
	b.userID = userID
	return b
}

func (b *TransactionBuilder) WithIdempotencyKey(key string) *TransactionBuilder {
	b.idempotencyKey = key
	return b
}

func (b *TransactionBuilder) Build() *entity.Transaction {
	now := time.Now()
	return &entity.Transaction{
		ID:                  uuid.New(),
		TransactionRef:      "TXN-" + uuid.NewString(),
		Type:                b.transactionType,
		Status:              b.status,
		SourceWalletID:      b.sourceWalletID,
		DestinationWalletID: b.destWalletID,
		Amount:              b.amount,
		Currency:            "IDR",
		IdempotencyKey:      b.idempotencyKey,
		InitiatedByUserID:   b.userID,
		Description:         b.description,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}
