package service

import "time"

type TaskStatus string

const (
	StatusTodo       TaskStatus = "todo"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
)

type Task struct {
	ID          int64
	TeamID      int64
	Title       string
	Description string
	Status      TaskStatus
	CreatedBy   int64
	AssigneeID  *int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ClosedAt    *time.Time
	Version     int64
}

type CreateTaskInput struct {
	TeamID      int64
	Title       string
	Description string
	Status      TaskStatus
	AssigneeID  *int64
}

type UpdateTaskInput struct {
	Version     int64
	Title       *string
	Description *string
	Status      *TaskStatus
	SetAssignee bool
	AssigneeID  *int64
}

type TaskFilter struct {
	TeamID     int64
	Status     TaskStatus
	AssigneeID *int64
	Limit      int
	Offset     int
}

type FieldChange struct {
	Old any
	New any
}

type TaskHistory struct {
	ID        int64
	TaskID    int64
	ChangedBy int64
	Changes   map[string]FieldChange
	CreatedAt time.Time
}

type Comment struct {
	ID        int64
	TaskID    int64
	UserID    int64
	Content   string
	CreatedAt time.Time
}

type CreateCommentInput struct{ Content string }
