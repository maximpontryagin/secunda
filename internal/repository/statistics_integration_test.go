//go:build integration

package repository_test

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"secunda/internal/repository"
	"secunda/internal/service"
)

func TestStatisticsReport(t *testing.T) {
	f := newTaskFixture(t)
	rollback := errors.New("rollback statistics fixtures")
	asOf := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	err := f.manager.WithinTransaction(f.ctx, func(ctx context.Context) error {
		users := repository.NewUsers(f.db)
		teams := repository.NewTeams(f.db)
		reports := repository.NewStatistics(f.db)
		empty, err := reports.Get(ctx, f.teamID, asOf)
		require.NoError(t, err)
		require.Equal(t, service.TaskCounts{}, empty.TasksByStatus)
		require.Empty(t, empty.TopAssignees)
		require.Nil(t, empty.AverageCloseSeconds)
		require.Zero(t, empty.CommentsCount)

		assignees := make([]int64, 4)
		for i := range assignees {
			user, err := users.Create(ctx, service.UserCredentials{User: service.User{Email: "report-" + rand.Text() + "@example.com", Name: "Assignee", CreatedAt: asOf}, PasswordHash: "unused-test-hash"})
			require.NoError(t, err)
			assignees[i] = user.ID
			require.NoError(t, teams.AddMember(ctx, service.Membership{TeamID: f.teamID, UserID: user.ID, Role: service.RoleMember}))
		}
		createTask := func(teamID int64, status service.TaskStatus, assignee *int64, closed time.Time, duration time.Duration) service.Task {
			task := service.Task{TeamID: teamID, Title: "Report task", Status: status, CreatedBy: f.userID, AssigneeID: assignee, CreatedAt: asOf.Add(-time.Hour), UpdatedAt: asOf, Version: 1}
			if status == service.StatusDone {
				task.ClosedAt = &closed
				task.CreatedAt = closed.Add(-duration)
			}
			task, err := f.tasks.Create(ctx, task)
			require.NoError(t, err)
			return task
		}
		addComments := func(taskID int64, count int) {
			for range count {
				_, err := f.tasks.AddComment(ctx, service.Comment{TaskID: taskID, UserID: f.userID, Content: "Comment", CreatedAt: asOf})
				require.NoError(t, err)
			}
		}
		var firstTask service.Task
		for i, count := range []int{4, 3, 2, 1} {
			for range count {
				task := createTask(f.teamID, service.StatusDone, &assignees[i], asOf.Add(-24*time.Hour), time.Duration(i+1)*time.Hour)
				if firstTask.ID == 0 {
					firstTask = task
				}
			}
		}
		createTask(f.teamID, service.StatusDone, &assignees[0], asOf.Add(-31*24*time.Hour), 10*time.Hour)
		createTask(f.teamID, service.StatusDone, &assignees[0], asOf.Add(-30*24*time.Hour), 5*time.Hour)
		createTask(f.teamID, service.StatusDone, nil, asOf.Add(-2*24*time.Hour), 6*time.Hour)
		todo := createTask(f.teamID, service.StatusTodo, nil, time.Time{}, 0)
		createTask(f.teamID, service.StatusTodo, nil, time.Time{}, 0)
		createTask(f.teamID, service.StatusInProgress, &assignees[0], time.Time{}, 0)
		addComments(firstTask.ID, 3)
		addComments(todo.ID, 2)

		foreign, err := teams.Create(ctx, service.Team{Name: "Foreign report team", CreatedBy: f.userID, CreatedAt: asOf})
		require.NoError(t, err)
		require.NoError(t, teams.AddMember(ctx, service.Membership{TeamID: foreign.ID, UserID: f.userID, Role: service.RoleOwner}))
		require.NoError(t, teams.AddMember(ctx, service.Membership{TeamID: foreign.ID, UserID: assignees[3], Role: service.RoleMember}))
		foreignTask := createTask(foreign.ID, service.StatusDone, &assignees[3], asOf.Add(-time.Hour), 100*time.Hour)
		createTask(foreign.ID, service.StatusDone, &assignees[3], asOf.Add(-time.Hour), 100*time.Hour)
		addComments(foreignTask.ID, 9)

		result, err := reports.Get(ctx, f.teamID, asOf)
		require.NoError(t, err)
		require.Equal(t, f.teamID, result.TeamID)
		require.Equal(t, service.TaskCounts{Todo: 2, InProgress: 1, Done: 13}, result.TasksByStatus)
		require.Equal(t, int64(5), result.CommentsCount)
		require.NotNil(t, result.AverageCloseSeconds)
		require.InDelta(t, 147600.0/13, *result.AverageCloseSeconds, 0.00001)
		require.Equal(t, []service.Performer{
			{UserID: assignees[0], Name: "Assignee", ClosedTasks: 5},
			{UserID: assignees[1], Name: "Assignee", ClosedTasks: 3},
			{UserID: assignees[2], Name: "Assignee", ClosedTasks: 2},
		}, result.TopAssignees)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
