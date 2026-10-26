package auth

import (
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"secunda/internal/service"
)

const issuer = "secunda"

type Tokens struct {
	secret []byte
	ttl    time.Duration
}

func NewTokens(secret string, ttl time.Duration) (*Tokens, error) {
	if len(secret) < 32 || ttl < time.Second {
		return nil, errors.New("JWT requires a secret of at least 32 bytes and TTL of at least one second")
	}
	return &Tokens{secret: []byte(secret), ttl: ttl}, nil
}

func (t *Tokens) Issue(userID int64) (service.AccessToken, error) {
	if userID <= 0 {
		return service.AccessToken{}, service.ErrUnauthorized
	}
	now := time.Now().UTC().Truncate(time.Second)
	expiresAt := now.Add(t.ttl).Truncate(time.Second)
	claims := jwt.RegisteredClaims{
		Issuer: issuer, Subject: strconv.FormatInt(userID, 10),
		IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expiresAt),
	}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	if err != nil {
		return service.AccessToken{}, err
	}
	return service.AccessToken{Value: value, ExpiresAt: expiresAt}, nil
}

func (t *Tokens) Verify(value string) (int64, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(value, claims, func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired(),
		jwt.WithIssuer(issuer), jwt.WithIssuedAt())
	if err != nil || !token.Valid {
		return 0, service.ErrUnauthorized
	}
	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || userID <= 0 {
		return 0, service.ErrUnauthorized
	}
	return userID, nil
}
