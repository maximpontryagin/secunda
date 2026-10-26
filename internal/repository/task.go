package repository

import (
	"context"
	"database/sql"

	"secunda/internal/platform/transaction"
	"secunda/internal/service"
)

type Tasks struct{ db *sql.DB }

func NewTasks(db *sql.DB) *Tasks { return &Tasks{db: db} }

const taskColumns = `id, team_id, title, description, status, created_by, assignee_id, created_at, updated_at, closed_at, version`

type rowScanner interface{ Scan(...any) error }

func scanTask(scanner rowScanner) (taskRow, error) {
	var row taskRow
	err := scanner.Scan(&row.ID, &row.TeamID, &row.Title, &row.Description, &row.Status, &row.CreatedBy,
		&row.AssigneeID, &row.CreatedAt, &row.UpdatedAt, &row.ClosedAt, &row.Version)
	return row, err
}

func (r *Tasks) Create(ctx context.Context, value service.Task) (service.Task, error) {
	row := taskRowFromService(value)
	result, err := transaction.Connection(ctx, r.db).ExecContext(ctx,
		`INSERT INTO tasks (team_id, title, description, status, created_by, assignee_id, created_at, updated_at, closed_at, version)
   VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, row.TeamID, row.Title, row.Description, row.Status, row.CreatedBy,
		row.AssigneeID, row.CreatedAt, row.UpdatedAt, row.ClosedAt, row.Version)
	if err != nil {
		return service.Task{}, databaseError("create task", err)
	}
	row.ID, err = result.LastInsertId()
	if err != nil {
		return service.Task{}, databaseError("read task id", err)
	}
	return row.toService(), nil
}

func (r *Tasks) Find(ctx context.Context, id int64) (service.Task, error) {
	row, err := scanTask(transaction.Connection(ctx, r.db).QueryRowContext(ctx, `SELECT `+taskColumns+` FROM tasks WHERE id = ?`, id))
	if err != nil {
		return service.Task{}, databaseError("find task", err)
	}
	return row.toService(), nil
}

func (r *Tasks) List(ctx context.Context, filter service.TaskFilter) ([]service.Task, error) {
	params := taskFilterRowFromService(filter)
	query := `SELECT ` + taskColumns + ` FROM tasks WHERE team_id = ?`
	args := []any{params.TeamID}
	if params.Status != "" {
		query += ` AND status = ?`
		args = append(args, params.Status)
	}
	if params.AssigneeID.Valid {
		query += ` AND assignee_id = ?`
		args = append(args, params.AssigneeID.Int64)
	}
	query += ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, params.Limit, params.Offset)
	rows, err := transaction.Connection(ctx, r.db).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, databaseError("list tasks", err)
	}
	defer rows.Close()
	tasks := make([]service.Task, 0)
	for rows.Next() {
		row, err := scanTask(rows)
		if err != nil {
			return nil, databaseError("scan task", err)
		}
		tasks = append(tasks, row.toService())
	}
	if err := rows.Err(); err != nil {
		return nil, databaseError("iterate tasks", err)
	}
	return tasks, nil
}

func (r *Tasks) Update(ctx context.Context, value service.Task, expectedVersion int64) error {
	row := taskRowFromService(value)
	result, err := transaction.Connection(ctx, r.db).ExecContext(ctx,
		`UPDATE tasks SET title = ?, description = ?, status = ?, assignee_id = ?, updated_at = ?, closed_at = ?, version = ?
   WHERE id = ? AND team_id = ? AND version = ?`, row.Title, row.Description, row.Status, row.AssigneeID,
		row.UpdatedAt, row.ClosedAt, row.Version, row.ID, row.TeamID, expectedVersion)
	if err != nil {
		return databaseError("update task", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return databaseError("read updated task count", err)
	}
	if count != 1 {
		return service.ErrVersionConflict
	}
	return nil
}

func (r *Tasks) AddHistory(ctx context.Context, value service.TaskHistory) error {
	row, err := historyRowFromService(value)
	if err != nil {
		return databaseError("encode task history", err)
	}
	_, err = transaction.Connection(ctx, r.db).ExecContext(ctx,
		`INSERT INTO task_history (task_id, changed_by, changes, created_at) VALUES (?, ?, ?, ?)`, row.TaskID, row.ChangedBy, row.Changes, row.CreatedAt)
	if err != nil {
		return databaseError("insert task history", err)
	}
	return nil
}

func (r *Tasks) ListHistory(ctx context.Context, taskID int64) ([]service.TaskHistory, error) {
	rows, err := transaction.Connection(ctx, r.db).QueryContext(ctx,
		`SELECT id, task_id, changed_by, changes, created_at FROM task_history WHERE task_id = ? ORDER BY id`, taskID)
	if err != nil {
		return nil, databaseError("list task history", err)
	}
	defer rows.Close()
	history := make([]service.TaskHistory, 0)
	for rows.Next() {
		var row historyRow
		if err := rows.Scan(&row.ID, &row.TaskID, &row.ChangedBy, &row.Changes, &row.CreatedAt); err != nil {
			return nil, databaseError("scan task history", err)
		}
		value, err := row.toService()
		if err != nil {
			return nil, databaseError("decode task history", err)
		}
		history = append(history, value)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseError("iterate task history", err)
	}
	return history, nil
}

func (r *Tasks) AddComment(ctx context.Context, value service.Comment) (service.Comment, error) {
	row := commentRowFromService(value)
	result, err := transaction.Connection(ctx, r.db).ExecContext(ctx,
		`INSERT INTO task_comments (task_id, user_id, content, created_at) VALUES (?, ?, ?, ?)`, row.TaskID, row.UserID, row.Content, row.CreatedAt)
	if err != nil {
		return service.Comment{}, databaseError("add comment", err)
	}
	row.ID, err = result.LastInsertId()
	if err != nil {
		return service.Comment{}, databaseError("read comment id", err)
	}
	return row.toService(), nil
}

func (r *Tasks) ListComments(ctx context.Context, taskID int64) ([]service.Comment, error) {
	rows, err := transaction.Connection(ctx, r.db).QueryContext(ctx,
		`SELECT id, task_id, user_id, content, created_at FROM task_comments WHERE task_id = ? ORDER BY id`, taskID)
	if err != nil {
		return nil, databaseError("list comments", err)
	}
	defer rows.Close()
	comments := make([]service.Comment, 0)
	for rows.Next() {
		var row commentRow
		if err := rows.Scan(&row.ID, &row.TaskID, &row.UserID, &row.Content, &row.CreatedAt); err != nil {
			return nil, databaseError("scan comment", err)
		}
		comments = append(comments, row.toService())
	}
	if err := rows.Err(); err != nil {
		return nil, databaseError("iterate comments", err)
	}
	return comments, nil
}

var _ service.TaskRepository = (*Tasks)(nil)

func (r *Tasks) ListVersion(ctx context.Context, teamID int64) (int64, error) {
	var version int64
	err := transaction.Connection(ctx, r.db).QueryRowContext(ctx, `SELECT task_list_version FROM teams WHERE id = ?`, teamID).Scan(&version)
	if err != nil {
		return 0, databaseError("get task list version", err)
	}
	return version, nil
}

func (r *Tasks) AdvanceListVersion(ctx context.Context, teamID int64) error {
	result, err := transaction.Connection(ctx, r.db).ExecContext(ctx, `UPDATE teams SET task_list_version = task_list_version + 1 WHERE id = ?`, teamID)
	if err != nil {
		return databaseError("advance task list version", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return databaseError("read task list version update count", err)
	}
	if affected != 1 {
		return service.ErrNotFound
	}
	return nil
}
