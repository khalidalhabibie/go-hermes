package auth

import (
	"fmt"
	"time"

	"go-hermes/internal/entity"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secretKey []byte
	issuer    string
	expiry    time.Duration
}

func NewJWTManager(secret, issuer string, expiryMinutes int) *JWTManager {
	return &JWTManager{
		secretKey: []byte(secret),
		issuer:    issuer,
		expiry:    time.Duration(expiryMinutes) * time.Minute,
	}
}

func (m *JWTManager) GenerateToken(user *entity.User) (string, time.Time, error) {
	expiresAt := time.Now().Add(m.expiry)
	claims := Claims{
		UserID: user.ID.String(),
		Role:   string(user.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   user.ID.String(),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secretKey)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

func (m *JWTManager) ParseToken(tokenString string) (*Claims, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
	)
	token, err := parser.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}
		return m.secretKey, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}
