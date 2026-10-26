package repository

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"time"

	"secunda/internal/service"
)

type taskRow struct {
	ID          int64
	TeamID      int64
	Title       string
	Description string
	Status      string
	CreatedBy   int64
	AssigneeID  sql.NullInt64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ClosedAt    sql.NullTime
	Version     int64
}

func taskRowFromService(value service.Task) taskRow {
	row := taskRow{ID: value.ID, TeamID: value.TeamID, Title: value.Title, Description: value.Description,
		Status: string(value.Status), CreatedBy: value.CreatedBy, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, Version: value.Version}
	if value.AssigneeID != nil {
		row.AssigneeID = sql.NullInt64{Int64: *value.AssigneeID, Valid: true}
	}
	if value.ClosedAt != nil {
		row.ClosedAt = sql.NullTime{Time: *value.ClosedAt, Valid: true}
	}
	return row
}

func (r taskRow) toService() service.Task {
	value := service.Task{ID: r.ID, TeamID: r.TeamID, Title: r.Title, Description: r.Description, Status: service.TaskStatus(r.Status),
		CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Version: r.Version}
	if r.AssigneeID.Valid {
		value.AssigneeID = &r.AssigneeID.Int64
	}
	if r.ClosedAt.Valid {
		value.ClosedAt = &r.ClosedAt.Time
	}
	return value
}

type taskFilterRow struct {
	TeamID     int64
	Status     string
	AssigneeID sql.NullInt64
	Limit      int
	Offset     int
}

func taskFilterRowFromService(value service.TaskFilter) taskFilterRow {
	row := taskFilterRow{TeamID: value.TeamID, Status: string(value.Status), Limit: value.Limit, Offset: value.Offset}
	if value.AssigneeID != nil {
		row.AssigneeID = sql.NullInt64{Int64: *value.AssigneeID, Valid: true}
	}
	return row
}

type changeRow struct {
	Old any `json:"old"`
	New any `json:"new"`
}

type historyRow struct {
	ID        int64
	TaskID    int64
	ChangedBy int64
	Changes   []byte
	CreatedAt time.Time
}

func historyRowFromService(value service.TaskHistory) (historyRow, error) {
	changes := make(map[string]changeRow, len(value.Changes))
	for field, change := range value.Changes {
		changes[field] = changeRow{Old: change.Old, New: change.New}
	}
	data, err := json.Marshal(changes)
	if err != nil {
		return historyRow{}, err
	}
	return historyRow{ID: value.ID, TaskID: value.TaskID, ChangedBy: value.ChangedBy, Changes: data, CreatedAt: value.CreatedAt}, nil
}

func (r historyRow) toService() (service.TaskHistory, error) {
	var changes map[string]changeRow
	decoder := json.NewDecoder(bytes.NewReader(r.Changes))
	decoder.UseNumber()
	if err := decoder.Decode(&changes); err != nil {
		return service.TaskHistory{}, err
	}
	value := service.TaskHistory{ID: r.ID, TaskID: r.TaskID, ChangedBy: r.ChangedBy, CreatedAt: r.CreatedAt, Changes: make(map[string]service.FieldChange, len(changes))}
	for field, change := range changes {
		value.Changes[field] = service.FieldChange{Old: change.Old, New: change.New}
	}
	return value, nil
}

type commentRow struct {
	ID        int64
	TaskID    int64
	UserID    int64
	Content   string
	CreatedAt time.Time
}

func commentRowFromService(value service.Comment) commentRow {
	return commentRow{ID: value.ID, TaskID: value.TaskID, UserID: value.UserID, Content: value.Content, CreatedAt: value.CreatedAt}
}

func (r commentRow) toService() service.Comment {
	return service.Comment{ID: r.ID, TaskID: r.TaskID, UserID: r.UserID, Content: r.Content, CreatedAt: r.CreatedAt}
}
