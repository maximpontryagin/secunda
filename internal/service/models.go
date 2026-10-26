package service

import "time"

type User struct {
	ID        int64
	Email     string
	Name      string
	CreatedAt time.Time
}

type UserCredentials struct {
	User         User
	PasswordHash string
}

type RegisterInput struct {
	Email    string
	Password string
	Name     string
}

type LoginInput struct {
	Email    string
	Password string
}

type AccessToken struct {
	Value     string
	ExpiresAt time.Time
}

type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

type Team struct {
	ID        int64
	Name      string
	CreatedBy int64
	CreatedAt time.Time
	Role      Role
}

type CreateTeamInput struct{ Name string }

type InviteInput struct {
	TeamID int64
	Email  string
	Role   Role
}

type Membership struct {
	TeamID int64
	UserID int64
	Role   Role
}
