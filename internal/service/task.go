package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

type TaskCache interface {
	Get(context.Context, TaskFilter, int64) ([]Task, bool, error)
	Put(context.Context, TaskFilter, int64, []Task) error
}

type TaskRepository interface {
	ListVersion(context.Context, int64) (int64, error)
	AdvanceListVersion(context.Context, int64) error
	Create(context.Context, Task) (Task, error)
	Find(context.Context, int64) (Task, error)
	List(context.Context, TaskFilter) ([]Task, error)
	Update(context.Context, Task, int64) error
	AddHistory(context.Context, TaskHistory) error
	ListHistory(context.Context, int64) ([]TaskHistory, error)
	AddComment(context.Context, Comment) (Comment, error)
	ListComments(context.Context, int64) ([]Comment, error)
}

type Tasks struct {
	tasks        TaskRepository
	teams        TeamRepository
	transactions TransactionManager
	cache        TaskCache
}

func NewTasks(tasks TaskRepository, teams TeamRepository, transactions TransactionManager, cache TaskCache) *Tasks {
	return &Tasks{tasks: tasks, teams: teams, transactions: transactions, cache: cache}
}

func (s *Tasks) Create(ctx context.Context, actorID int64, input CreateTaskInput) (Task, error) {
	if actorID <= 0 {
		return Task{}, ErrUnauthorized
	}
	if input.TeamID <= 0 {
		return Task{}, invalid("team_id must be positive")
	}
	title := strings.TrimSpace(input.Title)
	if !validName(title) {
		return Task{}, invalid("title must contain between 1 and 255 characters")
	}
	if !validText(input.Description) {
		return Task{}, invalid("description must contain at most 65535 bytes of UTF-8 text")
	}
	status := input.Status
	if status == "" {
		status = StatusTodo
	}
	if !validStatus(status) {
		return Task{}, invalid("status must be todo, in_progress or done")
	}
	var task Task
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		if _, err := s.teams.MemberRole(txCtx, input.TeamID, actorID); err != nil {
			return err
		}
		if err := s.checkAssignee(txCtx, input.TeamID, input.AssigneeID); err != nil {
			return err
		}
		if err := s.tasks.AdvanceListVersion(txCtx, input.TeamID); err != nil {
			return err
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		value := Task{TeamID: input.TeamID, Title: title, Description: input.Description, Status: status,
			CreatedBy: actorID, AssigneeID: input.AssigneeID, CreatedAt: now, UpdatedAt: now, Version: 1}
		if status == StatusDone {
			value.ClosedAt = &now
		}
		var err error
		task, err = s.tasks.Create(txCtx, value)
		if err != nil {
			return err
		}
		return s.tasks.AddHistory(txCtx, TaskHistory{
			TaskID: task.ID, ChangedBy: actorID, Changes: initialChanges(task), CreatedAt: now,
		})
	})
	if err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s *Tasks) List(ctx context.Context, actorID int64, filter TaskFilter) ([]Task, error) {
	if actorID <= 0 {
		return nil, ErrUnauthorized
	}
	if filter.TeamID <= 0 {
		return nil, invalid("team_id is required and must be positive")
	}
	if filter.Status != "" && !validStatus(filter.Status) {
		return nil, invalid("invalid status filter")
	}
	if filter.AssigneeID != nil && *filter.AssigneeID <= 0 {
		return nil, invalid("assignee_id must be positive")
	}
	if filter.Limit < 1 || filter.Limit > 100 || filter.Offset < 0 {
		return nil, invalid("limit must be between 1 and 100 and offset must be non-negative")
	}
	if _, err := s.teams.MemberRole(ctx, filter.TeamID, actorID); err != nil {
		return nil, err
	}
	if s.cache == nil {
		return s.tasks.List(ctx, filter)
	}
	version, err := s.tasks.ListVersion(ctx, filter.TeamID)
	if err != nil {
		return nil, err
	}
	cached, hit, cacheErr := s.cache.Get(ctx, filter, version)
	if cacheErr == nil && hit {
		return cached, nil
	}
	tasks, err := s.tasks.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	if cacheErr == nil {
		_ = s.cache.Put(ctx, filter, version, tasks)
	}
	return tasks, nil
}

func (s *Tasks) Update(ctx context.Context, actorID, taskID int64, input UpdateTaskInput) (Task, error) {
	if actorID <= 0 {
		return Task{}, ErrUnauthorized
	}
	if taskID <= 0 || input.Version <= 0 || input.Version == math.MaxInt64 {
		return Task{}, invalid("task id and version must be positive and version must allow an increment")
	}
	var updated Task
	err := s.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		current, role, err := s.accessibleTask(txCtx, actorID, taskID)
		if err != nil {
			return err
		}
		fullAccess := role == RoleOwner || role == RoleAdmin || current.CreatedBy == actorID
		if !fullAccess {
			if current.AssigneeID == nil || *current.AssigneeID != actorID {
				return ErrForbidden
			}
			if input.Title != nil || input.Description != nil || input.SetAssignee {
				return ErrForbidden
			}
		}
		if current.Version != input.Version {
			return ErrVersionConflict
		}
		next, changes, err := applyTaskUpdate(current, input)
		if err != nil {
			return err
		}
		if input.SetAssignee {
			if err := s.checkAssignee(txCtx, current.TeamID, next.AssigneeID); err != nil {
				return err
			}
		}
		if len(changes) == 0 {
			updated = current
			return nil
		}
		next.Version = current.Version + 1
		next.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)
		if current.Status != next.Status {
			if next.Status == StatusDone {
				next.ClosedAt = &next.UpdatedAt
			} else {
				next.ClosedAt = nil
			}
			changes["closed_at"] = FieldChange{Old: current.ClosedAt, New: next.ClosedAt}
		}
		changes["version"] = FieldChange{Old: current.Version, New: next.Version}
		changes["updated_at"] = FieldChange{Old: current.UpdatedAt, New: next.UpdatedAt}
		if err := s.tasks.AdvanceListVersion(txCtx, current.TeamID); err != nil {
			return err
		}
		if err := s.tasks.Update(txCtx, next, current.Version); err != nil {
			return err
		}
		if err := s.tasks.AddHistory(txCtx, TaskHistory{TaskID: next.ID, ChangedBy: actorID, Changes: changes, CreatedAt: next.UpdatedAt}); err != nil {
			return err
		}
		updated = next
		return nil
	})
	if err != nil {
		return Task{}, err
	}
	return updated, nil
}

func (s *Tasks) History(ctx context.Context, actorID, taskID int64) ([]TaskHistory, error) {
	if _, _, err := s.accessibleTask(ctx, actorID, taskID); err != nil {
		return nil, err
	}
	return s.tasks.ListHistory(ctx, taskID)
}

func (s *Tasks) AddComment(ctx context.Context, actorID, taskID int64, input CreateCommentInput) (Comment, error) {
	if _, _, err := s.accessibleTask(ctx, actorID, taskID); err != nil {
		return Comment{}, err
	}
	content := strings.TrimSpace(input.Content)
	if content == "" || !validText(content) {
		return Comment{}, invalid("content must contain between 1 and 65535 bytes of UTF-8 text")
	}
	return s.tasks.AddComment(ctx, Comment{TaskID: taskID, UserID: actorID, Content: content, CreatedAt: time.Now().UTC().Truncate(time.Microsecond)})
}

func (s *Tasks) Comments(ctx context.Context, actorID, taskID int64) ([]Comment, error) {
	if _, _, err := s.accessibleTask(ctx, actorID, taskID); err != nil {
		return nil, err
	}
	return s.tasks.ListComments(ctx, taskID)
}

func (s *Tasks) accessibleTask(ctx context.Context, actorID, taskID int64) (Task, Role, error) {
	if actorID <= 0 {
		return Task{}, "", ErrUnauthorized
	}
	if taskID <= 0 {
		return Task{}, "", invalid("task id must be positive")
	}
	task, err := s.tasks.Find(ctx, taskID)
	if err != nil {
		return Task{}, "", err
	}
	role, err := s.teams.MemberRole(ctx, task.TeamID, actorID)
	if err != nil {
		return Task{}, "", err
	}
	return task, role, nil
}

func (s *Tasks) checkAssignee(ctx context.Context, teamID int64, userID *int64) error {
	if userID == nil {
		return nil
	}
	if *userID <= 0 {
		return invalid("assignee_id must be positive or null")
	}
	_, err := s.teams.MemberRole(ctx, teamID, *userID)
	if errors.Is(err, ErrNotFound) {
		return invalid("assignee must be a member of the task's team")
	}
	return err
}

func validStatus(status TaskStatus) bool {
	return status == StatusTodo || status == StatusInProgress || status == StatusDone
}
func validText(value string) bool { return utf8.ValidString(value) && len(value) <= 65535 }
