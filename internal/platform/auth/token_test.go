package auth_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"secunda/internal/platform/auth"
	"secunda/internal/service"
)

const secret = "a-test-secret-with-at-least-32-bytes"

func TestTokenRoundTrip(t *testing.T) {
	tokens, err := auth.NewTokens(secret, time.Hour)
	require.NoError(t, err)
	token, err := tokens.Issue(42)
	require.NoError(t, err)
	id, err := tokens.Verify(token.Value)
	require.NoError(t, err)
	require.Equal(t, int64(42), id)
	require.WithinDuration(t, time.Now().Add(time.Hour), token.ExpiresAt, time.Second)
}

func TestTokenRejectsInvalidClaimsAndSignatures(t *testing.T) {
	cases := []struct {
		name   string
		change func(jwt.MapClaims)
		method jwt.SigningMethod
		key    string
	}{
		{name: "expired", change: func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() }},
		{name: "missing expiry", change: func(c jwt.MapClaims) { delete(c, "exp") }},
		{name: "wrong issuer", change: func(c jwt.MapClaims) { c["iss"] = "other" }},
		{name: "missing issuer", change: func(c jwt.MapClaims) { delete(c, "iss") }},
		{name: "non-numeric subject", change: func(c jwt.MapClaims) { c["sub"] = "user" }},
		{name: "zero subject", change: func(c jwt.MapClaims) { c["sub"] = "0" }},
		{name: "future issued at", change: func(c jwt.MapClaims) { c["iat"] = time.Now().Add(time.Hour).Unix() }},
		{name: "wrong algorithm", method: jwt.SigningMethodHS384},
		{name: "wrong signature", key: "another-secret-with-at-least-32-bytes"},
	}
	tokens, err := auth.NewTokens(secret, time.Hour)
	require.NoError(t, err)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			claims := jwt.MapClaims{"iss": "secunda", "sub": "42", "exp": time.Now().Add(time.Hour).Unix()}
			if tc.change != nil {
				tc.change(claims)
			}
			method := tc.method
			if method == nil {
				method = jwt.SigningMethodHS256
			}
			key := tc.key
			if key == "" {
				key = secret
			}
			value, err := jwt.NewWithClaims(method, claims).SignedString([]byte(key))
			require.NoError(t, err)
			_, err = tokens.Verify(value)
			require.ErrorIs(t, err, service.ErrUnauthorized)
		})
	}
	_, err = tokens.Verify("not-a-token")
	require.ErrorIs(t, err, service.ErrUnauthorized)
}
