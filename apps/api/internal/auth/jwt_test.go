package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestParseAccessTokenRejectsNonHS256Algorithm(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, Claims{UserID: "user", RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}})
	value, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAccessToken(secret, value); err == nil {
		t.Fatal("expected non-HS256 token to be rejected")
	}
}
