package repository

import (
	"time"

	"secunda/internal/service"
)

type userRow struct {
	ID           int64
	Email        string
	Name         string
	PasswordHash string
	CreatedAt    time.Time
}

func userRowFromService(value service.UserCredentials) userRow {
	return userRow{ID: value.User.ID, Email: value.User.Email, Name: value.User.Name, PasswordHash: value.PasswordHash, CreatedAt: value.User.CreatedAt}
}

func (r userRow) toService() service.UserCredentials {
	return service.UserCredentials{User: service.User{ID: r.ID, Email: r.Email, Name: r.Name, CreatedAt: r.CreatedAt}, PasswordHash: r.PasswordHash}
}

type teamRow struct {
	ID        int64
	Name      string
	CreatedBy int64
	CreatedAt time.Time
	Role      string
}

func teamRowFromService(value service.Team) teamRow {
	return teamRow{ID: value.ID, Name: value.Name, CreatedBy: value.CreatedBy, CreatedAt: value.CreatedAt, Role: string(value.Role)}
}

func (r teamRow) toService() service.Team {
	return service.Team{ID: r.ID, Name: r.Name, CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, Role: service.Role(r.Role)}
}

type memberRow struct {
	TeamID int64
	UserID int64
	Role   string
}

func memberRowFromService(value service.Membership) memberRow {
	return memberRow{TeamID: value.TeamID, UserID: value.UserID, Role: string(value.Role)}
}
