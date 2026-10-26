package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"secunda/internal/service"
)

const taskCacheTTL = 5 * time.Minute
const taskCacheTimeout = 200 * time.Millisecond

type TaskCache struct {
	client *redis.Client
	logger *slog.Logger
}

func NewTaskCache(client *redis.Client, logger *slog.Logger) *TaskCache {
	return &TaskCache{client: client, logger: logger}
}

func taskCacheKey(filter service.TaskFilter, version int64) string {
	assignee := "all"
	if filter.AssigneeID != nil {
		assignee = strconv.FormatInt(*filter.AssigneeID, 10)
	}
	return fmt.Sprintf("secunda:tasks:v1:team:%d:revision:%d:status:%s:assignee:%s:limit:%d:offset:%d",
		filter.TeamID, version, filter.Status, assignee, filter.Limit, filter.Offset)
}

func (c *TaskCache) Get(ctx context.Context, filter service.TaskFilter, version int64) ([]service.Task, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, taskCacheTimeout)
	defer cancel()
	data, err := c.client.Get(ctx, taskCacheKey(filter, version)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		c.logger.WarnContext(ctx, "task cache read failed", "error", err)
		return nil, false, err
	}
	var entries []taskCacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		c.logger.WarnContext(ctx, "task cache contains invalid JSON", "error", err)
		return nil, false, nil
	}
	tasks := make([]service.Task, 0, len(entries))
	for _, entry := range entries {
		tasks = append(tasks, entry.toService())
	}
	return tasks, true, nil
}

func (c *TaskCache) Put(ctx context.Context, filter service.TaskFilter, version int64, tasks []service.Task) error {
	entries := make([]taskCacheEntry, 0, len(tasks))
	for _, task := range tasks {
		entries = append(entries, taskCacheEntryFromService(task))
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("encode task cache: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, taskCacheTimeout)
	defer cancel()
	if err := c.client.Set(ctx, taskCacheKey(filter, version), data, taskCacheTTL).Err(); err != nil {
		c.logger.WarnContext(ctx, "task cache write failed", "error", err)
		return err
	}
	return nil
}

type taskCacheEntry struct {
	ID          int64      `json:"id"`
	TeamID      int64      `json:"team_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	CreatedBy   int64      `json:"created_by"`
	AssigneeID  *int64     `json:"assignee_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ClosedAt    *time.Time `json:"closed_at"`
	Version     int64      `json:"version"`
}

func taskCacheEntryFromService(task service.Task) taskCacheEntry {
	return taskCacheEntry{ID: task.ID, TeamID: task.TeamID, Title: task.Title, Description: task.Description, Status: string(task.Status),
		CreatedBy: task.CreatedBy, AssigneeID: task.AssigneeID, CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt, ClosedAt: task.ClosedAt, Version: task.Version}
}

func (e taskCacheEntry) toService() service.Task {
	return service.Task{ID: e.ID, TeamID: e.TeamID, Title: e.Title, Description: e.Description, Status: service.TaskStatus(e.Status),
		CreatedBy: e.CreatedBy, AssigneeID: e.AssigneeID, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, ClosedAt: e.ClosedAt, Version: e.Version}
}

var _ service.TaskCache = (*TaskCache)(nil)
