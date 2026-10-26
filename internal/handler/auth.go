package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"secunda/internal/service"
)

type AuthService interface {
	Register(context.Context, service.RegisterInput) (service.User, error)
	Login(context.Context, service.LoginInput) (service.AccessToken, error)
}

type TokenVerifier interface{ Verify(string) (int64, error) }

type Auth struct {
	service AuthService
	logger  *slog.Logger
}

func NewAuth(service AuthService, logger *slog.Logger) *Auth {
	return &Auth{service: service, logger: logger}
}

// Register godoc
// @Summary Register a user
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration"
// @Success 201 {object} UserResponse
// @Failure 400,409,413,415,500 {object} ErrorResponse
// @Router /register [post]
func (h *Auth) Register(w http.ResponseWriter, r *http.Request) {
	var request RegisterRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	user, err := h.service.Register(r.Context(), request.toService())
	if err != nil {
		serviceError(h.logger, w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, userResponse(user))
}

// Login godoc
// @Summary Authenticate and issue a JWT
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Credentials"
// @Success 200 {object} TokenResponse
// @Failure 400,401,413,415,500 {object} ErrorResponse
// @Router /login [post]
func (h *Auth) Login(w http.ResponseWriter, r *http.Request) {
	var request LoginRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	token, err := h.service.Login(r.Context(), request.toService())
	if err != nil {
		serviceError(h.logger, w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, tokenResponse(token))
}

type actorKey struct{}

func actorID(ctx context.Context) int64 {
	id, _ := ctx.Value(actorKey{}).(int64)
	return id
}

func authenticate(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			parts := strings.Fields(r.Header.Get("Authorization"))
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				unauthorized(w)
				return
			}
			userID, err := verifier.Verify(parts[1])
			if err != nil || userID <= 0 {
				unauthorized(w)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorKey{}, userID)))
		})
	}
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token required")
}
