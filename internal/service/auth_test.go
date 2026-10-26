package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mocks "secunda/internal/mocks/service"
	"secunda/internal/service"
)

func TestRegisterNormalizesEmailAndStoresHash(t *testing.T) {
	ctx := context.Background()
	users := mocks.NewUserRepository(t)
	passwords := mocks.NewPasswordHasher(t)
	tokens := mocks.NewTokenIssuer(t)
	passwords.EXPECT().Hash("password123").Return("bcrypt-hash", nil).Once()
	users.EXPECT().Create(ctx, mock.MatchedBy(func(value service.UserCredentials) bool {
		return value.User.Email == "alice@example.com" && value.User.Name == "Alice" && value.PasswordHash == "bcrypt-hash" && !value.User.CreatedAt.IsZero()
	})).Return(service.User{ID: 1, Email: "alice@example.com", Name: "Alice"}, nil).Once()
	result, err := service.NewAuth(users, passwords, tokens).Register(ctx,
		service.RegisterInput{Email: " ALICE@example.com ", Name: " Alice ", Password: "password123"})
	require.NoError(t, err)
	require.Equal(t, int64(1), result.ID)
}

func TestRegisterRejectsInvalidInputBeforeHashing(t *testing.T) {
	cases := []service.RegisterInput{
		{Email: "not-email", Name: "Alice", Password: "password123"},
		{Email: "Alice <alice@example.com>", Name: "Alice", Password: "password123"},
		{Email: "alice@example.com", Name: "  ", Password: "password123"},
		{Email: "alice@example.com", Name: "Alice", Password: "short"},
		{Email: "alice@example.com", Name: "Alice", Password: strings.Repeat("я", 37)},
	}
	for i, input := range cases {
		users := mocks.NewUserRepository(t)
		passwords := mocks.NewPasswordHasher(t)
		tokens := mocks.NewTokenIssuer(t)
		_, err := service.NewAuth(users, passwords, tokens).Register(context.Background(), input)
		require.ErrorIs(t, err, service.ErrInvalid, "case %d", i)
	}
}

func TestLoginHidesWhetherUserExists(t *testing.T) {
	for _, exists := range []bool{false, true} {
		users := mocks.NewUserRepository(t)
		passwords := mocks.NewPasswordHasher(t)
		tokens := mocks.NewTokenIssuer(t)
		credentials := service.UserCredentials{}
		lookupError := service.ErrNotFound
		if exists {
			credentials.PasswordHash = "stored-hash"
			lookupError = nil
		}
		users.EXPECT().FindByEmail(mock.Anything, "alice@example.com").Return(credentials, lookupError).Once()
		passwords.EXPECT().Compare(credentials.PasswordHash, "wrong-password").Return(errors.New("mismatch")).Once()
		result, err := service.NewAuth(users, passwords, tokens).Login(context.Background(),
			service.LoginInput{Email: "alice@example.com", Password: "wrong-password"})
		require.ErrorIs(t, err, service.ErrUnauthorized)
		require.Zero(t, result)
	}
}

func TestLoginIssuesTokenForStoredUser(t *testing.T) {
	users := mocks.NewUserRepository(t)
	passwords := mocks.NewPasswordHasher(t)
	tokens := mocks.NewTokenIssuer(t)
	users.EXPECT().FindByEmail(mock.Anything, "alice@example.com").Return(service.UserCredentials{User: service.User{ID: 42}, PasswordHash: "hash"}, nil).Once()
	passwords.EXPECT().Compare("hash", "password123").Return(nil).Once()
	tokens.EXPECT().Issue(int64(42)).Return(service.AccessToken{Value: "jwt"}, nil).Once()
	result, err := service.NewAuth(users, passwords, tokens).Login(context.Background(), service.LoginInput{Email: "ALICE@example.com", Password: "password123"})
	require.NoError(t, err)
	require.Equal(t, "jwt", result.Value)
}
