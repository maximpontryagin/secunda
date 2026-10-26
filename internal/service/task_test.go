package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mocks "secunda/internal/mocks/service"
	"secunda/internal/service"
)

func ptr[T any](value T) *T { return &value }

func taskService(t *testing.T) (*service.Tasks, *mocks.TaskRepository, *mocks.TeamRepository, *mocks.TransactionManager) {
	t.Helper()
	tasks := mocks.NewTaskRepository(t)
	teams := mocks.NewTeamRepository(t)
	tx := mocks.NewTransactionManager(t)
	return service.NewTasks(tasks, teams, tx, nil), tasks, teams, tx
}

func taskTransaction(tx *mocks.TransactionManager) {
	tx.EXPECT().WithinTransaction(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).Once()
}

func TestTaskUpdatePermissions(t *testing.T) {
	cases := []struct {
		name      string
		actor     int64
		role      service.Role
		input     service.UpdateTaskInput
		wantError error
	}{
		{name: "owner edits title", actor: 1, role: service.RoleOwner, input: service.UpdateTaskInput{Title: ptr("New")}},
		{name: "admin edits description", actor: 2, role: service.RoleAdmin, input: service.UpdateTaskInput{Description: ptr("New")}},
		{name: "creator edits title", actor: 3, role: service.RoleMember, input: service.UpdateTaskInput{Title: ptr("New")}},
		{name: "creator reassigns", actor: 3, role: service.RoleMember, input: service.UpdateTaskInput{SetAssignee: true, AssigneeID: ptr(int64(5))}},
		{name: "creator unassigns", actor: 3, role: service.RoleMember, input: service.UpdateTaskInput{SetAssignee: true}},
		{name: "assignee changes status", actor: 4, role: service.RoleMember, input: service.UpdateTaskInput{Status: ptr(service.StatusInProgress)}},
		{name: "assignee cannot edit title", actor: 4, role: service.RoleMember, input: service.UpdateTaskInput{Title: ptr("New")}, wantError: service.ErrForbidden},
		{name: "assignee cannot edit description", actor: 4, role: service.RoleMember, input: service.UpdateTaskInput{Description: ptr("New")}, wantError: service.ErrForbidden},
		{name: "assignee cannot unassign", actor: 4, role: service.RoleMember, input: service.UpdateTaskInput{SetAssignee: true}, wantError: service.ErrForbidden},
		{name: "assignee cannot reassign", actor: 4, role: service.RoleMember, input: service.UpdateTaskInput{SetAssignee: true, AssigneeID: ptr(int64(5))}, wantError: service.ErrForbidden},
		{name: "unrelated member cannot edit", actor: 5, role: service.RoleMember, input: service.UpdateTaskInput{Status: ptr(service.StatusDone)}, wantError: service.ErrForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, tasks, teams, tx := taskService(t)
			taskTransaction(tx)
			current := service.Task{ID: 10, TeamID: 20, Title: "Old", Status: service.StatusTodo, CreatedBy: 3, AssigneeID: ptr(int64(4)), Version: 7}
			tasks.EXPECT().Find(mock.Anything, int64(10)).Return(current, nil).Once()
			teams.EXPECT().MemberRole(mock.Anything, int64(20), tc.actor).Return(tc.role, nil).Once()
			if tc.wantError == nil {
				if tc.input.SetAssignee && tc.input.AssigneeID != nil {
					teams.EXPECT().MemberRole(mock.Anything, int64(20), *tc.input.AssigneeID).Return(service.RoleMember, nil).Once()
				}
				tasks.EXPECT().AdvanceListVersion(mock.Anything, int64(20)).Return(nil).Once()
				tasks.EXPECT().Update(mock.Anything, mock.MatchedBy(func(next service.Task) bool { return next.Version == 8 }), int64(7)).Return(nil).Once()
				tasks.EXPECT().AddHistory(mock.Anything, mock.MatchedBy(func(entry service.TaskHistory) bool {
					return entry.TaskID == 10 && entry.ChangedBy == tc.actor && entry.Changes["version"].Old == int64(7) && entry.Changes["version"].New == int64(8)
				})).Return(nil).Once()
			}
			tc.input.Version = 7
			result, err := svc.Update(context.Background(), tc.actor, 10, tc.input)
			if tc.wantError != nil {
				require.ErrorIs(t, err, tc.wantError)
				require.Zero(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, int64(8), result.Version)
			}
		})
	}
}

func TestTaskStatusClosedAt(t *testing.T) {
	now := time.Now().UTC().Add(-time.Hour)
	for _, reopen := range []bool{false, true} {
		svc, tasks, teams, tx := taskService(t)
		taskTransaction(tx)
		current := service.Task{ID: 10, TeamID: 20, CreatedBy: 1, Status: service.StatusTodo, Version: 1}
		nextStatus := service.StatusDone
		if reopen {
			current.Status = service.StatusDone
			current.ClosedAt = &now
			nextStatus = service.StatusTodo
		}
		tasks.EXPECT().Find(mock.Anything, int64(10)).Return(current, nil).Once()
		teams.EXPECT().MemberRole(mock.Anything, int64(20), int64(1)).Return(service.RoleMember, nil).Once()
		tasks.EXPECT().AdvanceListVersion(mock.Anything, int64(20)).Return(nil).Once()
		tasks.EXPECT().Update(mock.Anything, mock.MatchedBy(func(next service.Task) bool {
			if reopen {
				return next.ClosedAt == nil
			}
			return next.ClosedAt != nil && next.ClosedAt.Equal(next.UpdatedAt)
		}), int64(1)).Return(nil).Once()
		tasks.EXPECT().AddHistory(mock.Anything, mock.MatchedBy(func(entry service.TaskHistory) bool {
			_, exists := entry.Changes["closed_at"]
			return exists && entry.Changes["status"].New == nextStatus
		})).Return(nil).Once()
		_, err := svc.Update(context.Background(), 1, 10, service.UpdateTaskInput{Version: 1, Status: &nextStatus})
		require.NoError(t, err)
	}
}

func TestTaskUpdateFailuresDoNotWriteHistory(t *testing.T) {
	for _, scenario := range []string{"stale version", "concurrent update", "foreign assignee", "outsider"} {
		t.Run(scenario, func(t *testing.T) {
			svc, tasks, teams, tx := taskService(t)
			taskTransaction(tx)
			tasks.EXPECT().Find(mock.Anything, int64(10)).Return(service.Task{ID: 10, TeamID: 20, CreatedBy: 1, Title: "Old", Version: 2}, nil).Once()
			var accessErr error
			want := service.ErrVersionConflict
			if scenario == "outsider" {
				accessErr = service.ErrNotFound
				want = service.ErrNotFound
			}
			teams.EXPECT().MemberRole(mock.Anything, int64(20), int64(1)).Return(service.RoleMember, accessErr).Once()
			input := service.UpdateTaskInput{Version: 2, Title: ptr("New")}
			switch scenario {
			case "stale version":
				input.Version = 1
			case "concurrent update":
				tasks.EXPECT().AdvanceListVersion(mock.Anything, int64(20)).Return(nil).Once()
				tasks.EXPECT().Update(mock.Anything, mock.Anything, int64(2)).Return(service.ErrVersionConflict).Once()
			case "foreign assignee":
				input.SetAssignee = true
				input.AssigneeID = ptr(int64(99))
				want = service.ErrInvalid
				teams.EXPECT().MemberRole(mock.Anything, int64(20), int64(99)).Return(service.Role(""), service.ErrNotFound).Once()
			}
			result, err := svc.Update(context.Background(), 1, 10, input)
			require.ErrorIs(t, err, want)
			require.Zero(t, result)
		})
	}
}

func TestTaskUpdateHistoryFailurePropagatesToTransaction(t *testing.T) {
	svc, tasks, teams, tx := taskService(t)
	failure := errors.New("history insert failed")
	tx.EXPECT().WithinTransaction(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
		err := fn(ctx)
		require.ErrorIs(t, err, failure)
		return err
	}).Once()
	tasks.EXPECT().Find(mock.Anything, int64(10)).Return(service.Task{ID: 10, TeamID: 20, CreatedBy: 1, Version: 1}, nil).Once()
	teams.EXPECT().MemberRole(mock.Anything, int64(20), int64(1)).Return(service.RoleMember, nil).Once()
	tasks.EXPECT().AdvanceListVersion(mock.Anything, int64(20)).Return(nil).Once()
	tasks.EXPECT().Update(mock.Anything, mock.Anything, int64(1)).Return(nil).Once()
	tasks.EXPECT().AddHistory(mock.Anything, mock.Anything).Return(failure).Once()
	task, err := svc.Update(context.Background(), 1, 10, service.UpdateTaskInput{Version: 1, Title: ptr("New")})
	require.ErrorIs(t, err, failure)
	require.Zero(t, task)
}

func TestUnchangedTaskDoesNotIncrementVersion(t *testing.T) {
	svc, tasks, teams, tx := taskService(t)
	taskTransaction(tx)
	current := service.Task{ID: 10, TeamID: 20, Title: "Title", CreatedBy: 1, Version: 2}
	tasks.EXPECT().Find(mock.Anything, int64(10)).Return(current, nil).Once()
	teams.EXPECT().MemberRole(mock.Anything, int64(20), int64(1)).Return(service.RoleMember, nil).Once()
	result, err := svc.Update(context.Background(), 1, 10, service.UpdateTaskInput{Version: 2, Title: ptr("Title")})
	require.NoError(t, err)
	require.Equal(t, current, result)
}

func TestTaskCreateUsesTransactionAndWritesInitialHistory(t *testing.T) {
	svc, tasks, teams, tx := taskService(t)
	ctx := context.Background()
	txCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	tx.EXPECT().WithinTransaction(ctx, mock.Anything).RunAndReturn(func(_ context.Context, fn func(context.Context) error) error { return fn(txCtx) }).Once()
	teams.EXPECT().MemberRole(txCtx, int64(20), int64(1)).Return(service.RoleMember, nil).Once()
	teams.EXPECT().MemberRole(txCtx, int64(20), int64(2)).Return(service.RoleMember, nil).Once()
	tasks.EXPECT().AdvanceListVersion(txCtx, int64(20)).Return(nil).Once()
	tasks.EXPECT().Create(txCtx, mock.Anything).RunAndReturn(func(_ context.Context, task service.Task) (service.Task, error) {
		require.Equal(t, int64(1), task.CreatedBy)
		require.Equal(t, int64(1), task.Version)
		require.NotNil(t, task.ClosedAt)
		task.ID = 10
		return task, nil
	}).Once()
	tasks.EXPECT().AddHistory(txCtx, mock.MatchedBy(func(history service.TaskHistory) bool {
		return history.TaskID == 10 && history.ChangedBy == 1 && history.Changes["title"].Old == nil && history.Changes["title"].New == "Task"
	})).Return(nil).Once()
	result, err := svc.Create(ctx, 1, service.CreateTaskInput{TeamID: 20, Title: " Task ", Status: service.StatusDone, AssigneeID: ptr(int64(2))})
	require.NoError(t, err)
	require.Equal(t, int64(10), result.ID)
}

func TestTaskDataIsNotReturnedToOutsiders(t *testing.T) {
	for _, operation := range []string{"list", "history", "comments", "add comment"} {
		t.Run(operation, func(t *testing.T) {
			svc, tasks, teams, _ := taskService(t)
			if operation != "list" {
				tasks.EXPECT().Find(mock.Anything, int64(10)).Return(service.Task{ID: 10, TeamID: 20}, nil).Once()
			}
			teams.EXPECT().MemberRole(mock.Anything, int64(20), int64(99)).Return(service.Role(""), service.ErrNotFound).Once()
			var err error
			switch operation {
			case "list":
				_, err = svc.List(context.Background(), 99, service.TaskFilter{TeamID: 20, Limit: 20})
			case "history":
				_, err = svc.History(context.Background(), 99, 10)
			case "comments":
				_, err = svc.Comments(context.Background(), 99, 10)
			case "add comment":
				_, err = svc.AddComment(context.Background(), 99, 10, service.CreateCommentInput{Content: "Comment"})
			}
			require.ErrorIs(t, err, service.ErrNotFound)
		})
	}
}

func TestAnyTeamMemberCanComment(t *testing.T) {
	svc, tasks, teams, _ := taskService(t)
	tasks.EXPECT().Find(mock.Anything, int64(10)).Return(service.Task{ID: 10, TeamID: 20, CreatedBy: 1, AssigneeID: ptr(int64(2))}, nil).Once()
	teams.EXPECT().MemberRole(mock.Anything, int64(20), int64(3)).Return(service.RoleMember, nil).Once()
	tasks.EXPECT().AddComment(mock.Anything, mock.MatchedBy(func(comment service.Comment) bool {
		return comment.UserID == 3 && comment.TaskID == 10 && comment.Content == "Comment"
	})).Return(service.Comment{ID: 30}, nil).Once()
	result, err := svc.AddComment(context.Background(), 3, 10, service.CreateCommentInput{Content: " Comment "})
	require.NoError(t, err)
	require.Equal(t, int64(30), result.ID)
}
