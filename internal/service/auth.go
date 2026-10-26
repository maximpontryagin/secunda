package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

type UserRepository interface {
	Create(context.Context, UserCredentials) (User, error)
	FindByEmail(context.Context, string) (UserCredentials, error)
}

type PasswordHasher interface {
	Hash(string) (string, error)
	Compare(string, string) error
}

type TokenIssuer interface {
	Issue(int64) (AccessToken, error)
}

type Auth struct {
	users     UserRepository
	passwords PasswordHasher
	tokens    TokenIssuer
}

func NewAuth(users UserRepository, passwords PasswordHasher, tokens TokenIssuer) *Auth {
	return &Auth{users: users, passwords: passwords, tokens: tokens}
}

func (s *Auth) Register(ctx context.Context, input RegisterInput) (User, error) {
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return User{}, err
	}
	name := strings.TrimSpace(input.Name)
	if !validName(name) {
		return User{}, invalid("name must contain between 1 and 255 characters")
	}
	if len(input.Password) < 8 || len(input.Password) > 72 {
		return User{}, invalid("password must contain between 8 and 72 bytes")
	}
	hash, err := s.passwords.Hash(input.Password)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}
	return s.users.Create(ctx, UserCredentials{
		User:         User{Email: email, Name: name, CreatedAt: time.Now().UTC().Truncate(time.Microsecond)},
		PasswordHash: hash,
	})
}

func (s *Auth) Login(ctx context.Context, input LoginInput) (AccessToken, error) {
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return AccessToken{}, ErrUnauthorized
	}
	if len(input.Password) == 0 || len(input.Password) > 72 {
		return AccessToken{}, ErrUnauthorized
	}
	credentials, err := s.users.FindByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		_ = s.passwords.Compare("", input.Password)
		return AccessToken{}, ErrUnauthorized
	}
	if err != nil {
		return AccessToken{}, err
	}
	if err := s.passwords.Compare(credentials.PasswordHash, input.Password); err != nil {
		return AccessToken{}, ErrUnauthorized
	}
	token, err := s.tokens.Issue(credentials.User.ID)
	if err != nil {
		return AccessToken{}, fmt.Errorf("issue access token: %w", err)
	}
	return token, nil
}

func normalizeEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value || len(value) > 254 {
		return "", invalid("email must be a valid address of at most 254 bytes")
	}
	return value, nil
}

func validName(value string) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) >= 1 && utf8.RuneCountInString(value) <= 255
}
