package dto

type TopUpRequest struct {
	Amount      int64   `json:"amount" validate:"required,gt=0"`
	Description *string `json:"description"`
}

type TransferRequest struct {
	RecipientWalletID string  `json:"recipient_wallet_id" validate:"required,uuid4"`
	Amount            int64   `json:"amount" validate:"required,gt=0"`
	Description       *string `json:"description"`
}
