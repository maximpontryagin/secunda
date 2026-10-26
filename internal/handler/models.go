package handler

import (
	"time"

	"secunda/internal/service"
)

type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email" maxLength:"254" example:"alice@example.com"`
	Password string `json:"password" validate:"required" minLength:"8" maxLength:"72" example:"local-password"`
	Name     string `json:"name" validate:"required" minLength:"1" maxLength:"255" example:"Alice"`
}

func (r RegisterRequest) toService() service.RegisterInput {
	return service.RegisterInput{Email: r.Email, Password: r.Password, Name: r.Name}
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email" example:"alice@example.com"`
	Password string `json:"password" validate:"required" example:"local-password"`
}

func (r LoginRequest) toService() service.LoginInput {
	return service.LoginInput{Email: r.Email, Password: r.Password}
}

type UserResponse struct {
	ID        int64     `json:"id" example:"1"`
	Email     string    `json:"email" example:"alice@example.com"`
	Name      string    `json:"name" example:"Alice"`
	CreatedAt time.Time `json:"created_at"`
}

func userResponse(value service.User) UserResponse {
	return UserResponse{ID: value.ID, Email: value.Email, Name: value.Name, CreatedAt: value.CreatedAt}
}

type TokenResponse struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type" example:"Bearer"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func tokenResponse(value service.AccessToken) TokenResponse {
	return TokenResponse{AccessToken: value.Value, TokenType: "Bearer", ExpiresAt: value.ExpiresAt}
}

type CreateTeamRequest struct {
	Name string `json:"name" validate:"required" minLength:"1" maxLength:"255" example:"Backend"`
}

func (r CreateTeamRequest) toService() service.CreateTeamInput {
	return service.CreateTeamInput{Name: r.Name}
}

type TeamResponse struct {
	ID        int64     `json:"id" example:"1"`
	Name      string    `json:"name" example:"Backend"`
	CreatedBy int64     `json:"created_by" example:"1"`
	CreatedAt time.Time `json:"created_at"`
	Role      string    `json:"role" enums:"owner,admin,member" example:"owner"`
}

func teamResponse(value service.Team) TeamResponse {
	return TeamResponse{ID: value.ID, Name: value.Name, CreatedBy: value.CreatedBy, CreatedAt: value.CreatedAt, Role: string(value.Role)}
}

type TeamsResponse struct {
	Items []TeamResponse `json:"items"`
}

func teamsResponse(values []service.Team) TeamsResponse {
	items := make([]TeamResponse, 0, len(values))
	for _, value := range values {
		items = append(items, teamResponse(value))
	}
	return TeamsResponse{Items: items}
}

type InviteRequest struct {
	Email string `json:"email" validate:"required,email" maxLength:"254" example:"bob@example.com"`
	Role  string `json:"role" enums:"admin,member" default:"member" example:"member"`
}

func (r InviteRequest) toService(teamID int64) service.InviteInput {
	return service.InviteInput{TeamID: teamID, Email: r.Email, Role: service.Role(r.Role)}
}

type MembershipResponse struct {
	TeamID int64  `json:"team_id" example:"1"`
	UserID int64  `json:"user_id" example:"2"`
	Role   string `json:"role" enums:"admin,member" example:"member"`
}

func membershipResponse(value service.Membership) MembershipResponse {
	return MembershipResponse{TeamID: value.TeamID, UserID: value.UserID, Role: string(value.Role)}
}

type ErrorResponse struct {
	Code    string `json:"code" example:"invalid_request"`
	Message string `json:"message" example:"invalid request"`
}
