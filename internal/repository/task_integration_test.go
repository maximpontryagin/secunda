//go:build integration

package repository_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"

	"secunda/internal/config"
	"secunda/internal/platform/mysql"
	"secunda/internal/platform/transaction"
	"secunda/internal/repository"
	"secunda/internal/service"
)

type taskFixture struct {
	db      *sql.DB
	userID  int64
	teamID  int64
	ctx     context.Context
	tasks   *repository.Tasks
	manager *transaction.Manager
	service *service.Tasks
}

func newTaskFixture(t *testing.T) taskFixture {
	t.Helper()
	_ = godotenv.Load("../../.env")
	cfg, err := config.LoadMySQL()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	db, err := mysql.Open(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	manager := transaction.New(db)
	users := repository.NewUsers(db)
	teams := repository.NewTeams(db)
	fixture := taskFixture{db: db, ctx: ctx, tasks: repository.NewTasks(db), manager: manager}
	err = manager.WithinTransaction(ctx, func(ctx context.Context) error {
		user, err := users.Create(ctx, service.UserCredentials{
			User: service.User{Email: "tasks-" + rand.Text() + "@example.com", Name: "Task test", CreatedAt: time.Now().UTC()}, PasswordHash: "unused-test-hash",
		})
		if err != nil {
			return err
		}
		fixture.userID = user.ID
		team, err := teams.Create(ctx, service.Team{Name: "Task test", CreatedBy: user.ID, CreatedAt: time.Now().UTC()})
		if err != nil {
			return err
		}
		fixture.teamID = team.ID
		return teams.AddMember(ctx, service.Membership{TeamID: team.ID, UserID: user.ID, Role: service.RoleOwner})
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		statements := []struct {
			query string
			id    int64
		}{
			{`DELETE h FROM task_history h JOIN tasks t ON t.id = h.task_id WHERE t.team_id = ?`, fixture.teamID},
			{`DELETE c FROM task_comments c JOIN tasks t ON t.id = c.task_id WHERE t.team_id = ?`, fixture.teamID},
			{`DELETE FROM tasks WHERE team_id = ?`, fixture.teamID},
			{`DELETE FROM team_members WHERE team_id = ?`, fixture.teamID},
			{`DELETE FROM teams WHERE id = ?`, fixture.teamID},
			{`DELETE FROM users WHERE id = ?`, fixture.userID},
		}
		for _, statement := range statements {
			_, err := db.ExecContext(cleanup, statement.query, statement.id)
			require.NoError(t, err)
		}
	})
	fixture.service = service.NewTasks(fixture.tasks, teams, manager)
	return fixture
}

func TestConcurrentTaskUpdateHasOneWinner(t *testing.T) {
	f := newTaskFixture(t)
	task, err := f.service.Create(f.ctx, f.userID, service.CreateTaskInput{TeamID: f.teamID, Title: "Original"})
	require.NoError(t, err)
	start := make(chan struct{})
	type result struct {
		task service.Task
		err  error
	}
	results := make(chan result, 2)
	for _, title := range []string{"First", "Second"} {
		go func(title string) {
			<-start
			task, err := f.service.Update(f.ctx, f.userID, task.ID, service.UpdateTaskInput{Version: 1, Title: &title})
			results <- result{task: task, err: err}
		}(title)
	}
	close(start)
	var winner service.Task
	successes, conflicts := 0, 0
	for range 2 {
		value := <-results
		if value.err == nil {
			successes++
			winner = value.task
		} else {
			require.ErrorIs(t, value.err, service.ErrVersionConflict)
			conflicts++
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
	stored, err := f.tasks.Find(f.ctx, task.ID)
	require.NoError(t, err)
	require.Equal(t, winner.Title, stored.Title)
	require.Equal(t, int64(2), stored.Version)
	history, err := f.tasks.ListHistory(f.ctx, task.ID)
	require.NoError(t, err)
	require.Len(t, history, 2)
	require.Equal(t, "Original", history[1].Changes["title"].Old)
	require.Equal(t, winner.Title, history[1].Changes["title"].New)
	require.Equal(t, json.Number("1"), history[1].Changes["version"].Old)
	require.Equal(t, json.Number("2"), history[1].Changes["version"].New)
}

func TestHistoryFailureRollsBackTaskCreationAndUpdate(t *testing.T) {
	f := newTaskFixture(t)
	now := time.Now().UTC()
	var failedID int64
	err := f.manager.WithinTransaction(f.ctx, func(ctx context.Context) error {
		task, err := f.tasks.Create(ctx, service.Task{TeamID: f.teamID, Title: "Must rollback", Status: service.StatusTodo, CreatedBy: f.userID, CreatedAt: now, UpdatedAt: now, Version: 1})
		if err != nil {
			return err
		}
		failedID = task.ID
		return f.tasks.AddHistory(ctx, service.TaskHistory{TaskID: task.ID, ChangedBy: -1, Changes: map[string]service.FieldChange{}, CreatedAt: now})
	})
	require.Error(t, err)
	require.Positive(t, failedID)
	_, err = f.tasks.Find(f.ctx, failedID)
	require.ErrorIs(t, err, service.ErrNotFound)

	task, err := f.service.Create(f.ctx, f.userID, service.CreateTaskInput{TeamID: f.teamID, Title: "Original"})
	require.NoError(t, err)
	updated := task
	updated.Title = "Must rollback"
	updated.Version++
	err = f.manager.WithinTransaction(f.ctx, func(ctx context.Context) error {
		if err := f.tasks.Update(ctx, updated, task.Version); err != nil {
			return err
		}
		return f.tasks.AddHistory(ctx, service.TaskHistory{TaskID: task.ID, ChangedBy: -1, Changes: map[string]service.FieldChange{}, CreatedAt: now})
	})
	require.Error(t, err)
	require.False(t, errors.Is(err, service.ErrVersionConflict))
	stored, err := f.tasks.Find(f.ctx, task.ID)
	require.NoError(t, err)
	require.Equal(t, task, stored)
	history, err := f.tasks.ListHistory(f.ctx, task.ID)
	require.NoError(t, err)
	require.Len(t, history, 1)
}
