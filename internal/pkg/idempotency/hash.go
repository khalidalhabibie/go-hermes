package idempotency

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func HashPayload(payload interface{}) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
