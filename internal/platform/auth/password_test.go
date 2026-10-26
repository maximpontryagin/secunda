package auth_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"secunda/internal/platform/auth"
)

func TestPasswords(t *testing.T) {
	passwords, err := auth.NewPasswords()
	require.NoError(t, err)
	hash, err := passwords.Hash("password123")
	require.NoError(t, err)
	require.NotEqual(t, "password123", hash)
	require.NoError(t, passwords.Compare(hash, "password123"))
	require.Error(t, passwords.Compare(hash, "wrong-password"))
	require.Error(t, passwords.Compare("", "wrong-password"))
}
