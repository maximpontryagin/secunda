//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"secunda/internal/config"
	platformredis "secunda/internal/platform/redis"
	"secunda/internal/repository"
	"secunda/internal/service"
)

func TestTaskCacheVersionsAndRecovery(t *testing.T) {
	f := newTaskFixture(t)
	cfg, err := config.Load()
	require.NoError(t, err)
	client, err := platformredis.Open(f.ctx, cfg.Redis)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	pattern := fmt.Sprintf("secunda:tasks:v1:team:%d:*", f.teamID)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		keys := client.Scan(ctx, 0, pattern, 100).Iterator()
		for keys.Next(ctx) {
			require.NoError(t, client.Del(ctx, keys.Val()).Err())
		}
		require.NoError(t, keys.Err())
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cache := repository.NewTaskCache(client, logger)
	cached := service.NewTasks(f.tasks, repository.NewTeams(f.db), f.manager, cache)
	filter := service.TaskFilter{TeamID: f.teamID, Limit: 20}
	task, err := cached.Create(f.ctx, f.userID, service.CreateTaskInput{TeamID: f.teamID, Title: "Original"})
	require.NoError(t, err)
	oldVersion, err := f.tasks.ListVersion(f.ctx, f.teamID)
	require.NoError(t, err)
	oldTasks, err := cached.List(f.ctx, f.userID, filter)
	require.NoError(t, err)
	stored, hit, err := cache.Get(f.ctx, filter, oldVersion)
	require.NoError(t, err)
	require.True(t, hit)
	require.Equal(t, oldTasks, stored)
	iterator := client.Scan(f.ctx, 0, pattern, 100).Iterator()
	keyCount := 0
	for iterator.Next(f.ctx) {
		ttl, err := client.TTL(f.ctx, iterator.Val()).Result()
		require.NoError(t, err)
		require.Greater(t, ttl, 290*time.Second)
		require.LessOrEqual(t, ttl, 300*time.Second)
		keyCount++
	}
	require.NoError(t, iterator.Err())
	require.Equal(t, 1, keyCount)

	doneFilter := filter
	doneFilter.Status = service.StatusDone
	empty, err := cached.List(f.ctx, f.userID, doneFilter)
	require.NoError(t, err)
	require.Empty(t, empty)
	_, hit, err = cache.Get(f.ctx, doneFilter, oldVersion)
	require.NoError(t, err)
	require.True(t, hit, "empty lists are cached")

	newTitle := "Changed"
	_, err = cached.Update(f.ctx, f.userID, task.ID, service.UpdateTaskInput{Version: 1, Title: &newTitle})
	require.NoError(t, err)
	newVersion, err := f.tasks.ListVersion(f.ctx, f.teamID)
	require.NoError(t, err)
	require.Equal(t, oldVersion+1, newVersion)
	// A reader that started before the commit finishes filling its old cache key.
	require.NoError(t, cache.Put(f.ctx, filter, oldVersion, oldTasks))
	current, err := cached.List(f.ctx, f.userID, filter)
	require.NoError(t, err)
	require.Equal(t, "Changed", current[0].Title)

	closedClient := redis.NewClient(&redis.Options{Addr: client.Options().Addr})
	require.NoError(t, closedClient.Close())
	unavailable := repository.NewTaskCache(closedClient, logger)
	fallback := service.NewTasks(f.tasks, repository.NewTeams(f.db), f.manager, unavailable)
	anotherTitle := "Written while Redis unavailable"
	_, err = fallback.Update(f.ctx, f.userID, task.ID, service.UpdateTaskInput{Version: 2, Title: &anotherTitle})
	require.NoError(t, err)
	fromDatabase, err := fallback.List(f.ctx, f.userID, filter)
	require.NoError(t, err)
	require.Equal(t, anotherTitle, fromDatabase[0].Title)
	afterRecovery, err := cached.List(f.ctx, f.userID, filter)
	require.NoError(t, err)
	require.Equal(t, anotherTitle, afterRecovery[0].Title)

	second, err := cached.Create(f.ctx, f.userID, service.CreateTaskInput{TeamID: f.teamID, Title: "Second", Status: service.StatusDone, AssigneeID: &f.userID})
	require.NoError(t, err)
	pageFilter := filter
	pageFilter.Limit = 1
	pageFilter.Offset = 1
	page, err := cached.List(f.ctx, f.userID, pageFilter)
	require.NoError(t, err)
	require.Len(t, page, 1)
	require.Equal(t, task.ID, page[0].ID)
	assignedFilter := filter
	assignedFilter.AssigneeID = &f.userID
	assigned, err := cached.List(f.ctx, f.userID, assignedFilter)
	require.NoError(t, err)
	require.Len(t, assigned, 1)
	require.Equal(t, second.ID, assigned[0].ID)
	closed, err := cached.List(f.ctx, f.userID, doneFilter)
	require.NoError(t, err)
	require.Len(t, closed, 1)
	require.Equal(t, second.ID, closed[0].ID)
}
