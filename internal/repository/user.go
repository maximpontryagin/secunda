package repository

import (
	"context"
	"database/sql"

	"secunda/internal/platform/transaction"
	"secunda/internal/service"
)

type Users struct{ db *sql.DB }

func NewUsers(db *sql.DB) *Users { return &Users{db: db} }

func (r *Users) Create(ctx context.Context, value service.UserCredentials) (service.User, error) {
	row := userRowFromService(value)
	result, err := transaction.Connection(ctx, r.db).ExecContext(ctx,
		`INSERT INTO users (email, password_hash, name, created_at) VALUES (?, ?, ?, ?)`,
		row.Email, row.PasswordHash, row.Name, row.CreatedAt)
	if err != nil {
		return service.User{}, databaseError("create user", err)
	}
	row.ID, err = result.LastInsertId()
	if err != nil {
		return service.User{}, databaseError("read user id", err)
	}
	return row.toService().User, nil
}

func (r *Users) FindByEmail(ctx context.Context, email string) (service.UserCredentials, error) {
	var row userRow
	err := transaction.Connection(ctx, r.db).QueryRowContext(ctx,
		`SELECT id, email, password_hash, name, created_at FROM users WHERE email = ?`, email).
		Scan(&row.ID, &row.Email, &row.PasswordHash, &row.Name, &row.CreatedAt)
	if err != nil {
		return service.UserCredentials{}, databaseError("find user by email", err)
	}
	return row.toService(), nil
}

var _ service.UserRepository = (*Users)(nil)
