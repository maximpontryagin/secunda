package handler_test

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"secunda/internal/handler"
	mocks "secunda/internal/mocks/handler"
	"secunda/internal/service"
)

func testRouter(t *testing.T) (http.Handler, *mocks.AuthService, *mocks.TeamService, *mocks.TokenVerifier) {
	t.Helper()
	auth := mocks.NewAuthService(t)
	teams := mocks.NewTeamService(t)
	verifier := mocks.NewTokenVerifier(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return handler.NewRouter(handler.NewAuth(auth, logger), handler.NewTeams(teams, logger), verifier), auth, teams, verifier
}

func TestProtectedRoutesRequireBearerToken(t *testing.T) {
	for _, route := range []struct{ method, path string }{
		{"GET", "/api/v1/teams"}, {"POST", "/api/v1/teams"}, {"POST", "/api/v1/teams/1/invite"},
	} {
		router, _, _, _ := testRouter(t)
		request := httptest.NewRequest(route.method, route.path, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, http.StatusUnauthorized, response.Code)
		require.Equal(t, "Bearer", response.Header().Get("WWW-Authenticate"))
	}
}

func TestTeamListUsesVerifiedIdentity(t *testing.T) {
	router, _, teams, verifier := testRouter(t)
	verifier.EXPECT().Verify("valid-token").Return(int64(42), nil).Once()
	teams.EXPECT().List(mock.Anything, int64(42)).Return(nil, nil).Once()
	request := httptest.NewRequest("GET", "/api/v1/teams?user_id=1", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"items":[]}`, response.Body.String())
}

func TestInvalidTokenDoesNotReachService(t *testing.T) {
	router, _, _, verifier := testRouter(t)
	verifier.EXPECT().Verify("invalid-token").Return(int64(0), service.ErrUnauthorized).Once()
	request := httptest.NewRequest("GET", "/api/v1/teams", nil)
	request.Header.Set("Authorization", "Bearer invalid-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestRegisterRejectsMalformedBody(t *testing.T) {
	cases := []struct {
		name, body, contentType string
		status                  int
	}{
		{"unknown field", `{"email":"alice@example.com","role":"owner"}`, "application/json", 400},
		{"multiple values", `{} {}`, "application/json", 400},
		{"array", `[]`, "application/json", 400},
		{"empty body", ``, "application/json", 400},
		{"wrong content type", `{}`, "text/plain", 415},
		{"oversized body", `{"name":"` + strings.Repeat("a", 1<<20) + `"}`, "application/json", 413},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router, _, _, _ := testRouter(t)
			request := httptest.NewRequest("POST", "/api/v1/register", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", tc.contentType)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, tc.status, response.Code)
		})
	}
}

func TestRegisterResponse(t *testing.T) {
	router, auth, _, _ := testRouter(t)
	auth.EXPECT().Register(mock.Anything, service.RegisterInput{Email: "alice@example.com", Name: "Alice", Password: "password123"}).
		Return(service.User{ID: 1, Email: "alice@example.com", Name: "Alice"}, nil).Once()
	request := httptest.NewRequest("POST", "/api/v1/register", strings.NewReader(`{"email":"alice@example.com","name":"Alice","password":"password123"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusCreated, response.Code)
	require.Contains(t, response.Body.String(), `"id":1`)
	require.NotContains(t, response.Body.String(), "password")
}

func TestInternalErrorIsNotExposed(t *testing.T) {
	router, auth, _, _ := testRouter(t)
	auth.EXPECT().Login(mock.Anything, mock.Anything).Return(service.AccessToken{}, errors.New("private database details")).Once()
	request := httptest.NewRequest("POST", "/api/v1/login", strings.NewReader(`{"email":"alice@example.com","password":"password123"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.NotContains(t, response.Body.String(), "private")
}
