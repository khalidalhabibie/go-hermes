package auth

import (
	"testing"
	"time"

	"go-hermes/internal/entity"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestParseTokenAcceptsExpectedIssuer(t *testing.T) {
	manager := NewJWTManager("01234567890123456789012345678901", "go-hermes", 60)
	user := &entity.User{
		ID:   uuid.New(),
		Role: entity.RoleUser,
	}

	token, _, err := manager.GenerateToken(user)
	require.NoError(t, err)

	claims, err := manager.ParseToken(token)

	require.NoError(t, err)
	require.Equal(t, user.ID.String(), claims.UserID)
	require.Equal(t, "go-hermes", claims.Issuer)
}

func TestParseTokenRejectsUnexpectedIssuer(t *testing.T) {
	manager := NewJWTManager("01234567890123456789012345678901", "go-hermes", 60)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID: uuid.NewString(),
		Role:   string(entity.RoleUser),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "other-issuer",
			Subject:   uuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})
	signed, err := token.SignedString([]byte("01234567890123456789012345678901"))
	require.NoError(t, err)

	claims, err := manager.ParseToken(signed)

	require.Nil(t, claims)
	require.Error(t, err)
}
