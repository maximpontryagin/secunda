package repository

import (
	"context"
	"database/sql"

	"secunda/internal/platform/transaction"
	"secunda/internal/service"
)

type Teams struct{ db *sql.DB }

func NewTeams(db *sql.DB) *Teams { return &Teams{db: db} }

func (r *Teams) Create(ctx context.Context, value service.Team) (service.Team, error) {
	row := teamRowFromService(value)
	result, err := transaction.Connection(ctx, r.db).ExecContext(ctx,
		`INSERT INTO teams (name, created_by, created_at) VALUES (?, ?, ?)`, row.Name, row.CreatedBy, row.CreatedAt)
	if err != nil {
		return service.Team{}, databaseError("create team", err)
	}
	row.ID, err = result.LastInsertId()
	if err != nil {
		return service.Team{}, databaseError("read team id", err)
	}
	return row.toService(), nil
}

func (r *Teams) AddMember(ctx context.Context, value service.Membership) error {
	row := memberRowFromService(value)
	_, err := transaction.Connection(ctx, r.db).ExecContext(ctx,
		`INSERT INTO team_members (team_id, user_id, role) VALUES (?, ?, ?)`, row.TeamID, row.UserID, row.Role)
	if err != nil {
		return databaseError("add team member", err)
	}
	return nil
}

func (r *Teams) ListByUser(ctx context.Context, userID int64) ([]service.Team, error) {
	rows, err := transaction.Connection(ctx, r.db).QueryContext(ctx,
		`SELECT t.id, t.name, t.created_by, t.created_at, m.role
   FROM teams t JOIN team_members m ON m.team_id = t.id
   WHERE m.user_id = ? ORDER BY t.id DESC`, userID)
	if err != nil {
		return nil, databaseError("list teams", err)
	}
	defer rows.Close()
	teams := make([]service.Team, 0)
	for rows.Next() {
		var row teamRow
		if err := rows.Scan(&row.ID, &row.Name, &row.CreatedBy, &row.CreatedAt, &row.Role); err != nil {
			return nil, databaseError("scan team", err)
		}
		teams = append(teams, row.toService())
	}
	if err := rows.Err(); err != nil {
		return nil, databaseError("iterate teams", err)
	}
	return teams, nil
}

func (r *Teams) MemberRole(ctx context.Context, teamID, userID int64) (service.Role, error) {
	var row memberRow
	err := transaction.Connection(ctx, r.db).QueryRowContext(ctx,
		`SELECT team_id, user_id, role FROM team_members WHERE team_id = ? AND user_id = ?`, teamID, userID).
		Scan(&row.TeamID, &row.UserID, &row.Role)
	if err != nil {
		return "", databaseError("get team member role", err)
	}
	return service.Role(row.Role), nil
}

var _ service.TeamRepository = (*Teams)(nil)
