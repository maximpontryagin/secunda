package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mocks "secunda/internal/mocks/service"
	"secunda/internal/service"
)

func TestCachedTasksAlwaysCheckMembership(t *testing.T) {
	tasks := mocks.NewTaskRepository(t)
	teams := mocks.NewTeamRepository(t)
	tx := mocks.NewTransactionManager(t)
	cache := mocks.NewTaskCache(t)
	teams.EXPECT().MemberRole(mock.Anything, int64(10), int64(99)).Return(service.Role(""), service.ErrNotFound).Once()
	_, err := service.NewTasks(tasks, teams, tx, cache).List(context.Background(), 99, service.TaskFilter{TeamID: 10, Limit: 20})
	require.ErrorIs(t, err, service.ErrNotFound)
}

func TestTaskCacheHitAndFallback(t *testing.T) {
	for _, scenario := range []string{"hit", "miss", "read failure", "write failure"} {
		t.Run(scenario, func(t *testing.T) {
			tasks := mocks.NewTaskRepository(t)
			teams := mocks.NewTeamRepository(t)
			tx := mocks.NewTransactionManager(t)
			cache := mocks.NewTaskCache(t)
			filter := service.TaskFilter{TeamID: 10, Limit: 20, Status: service.StatusTodo}
			values := []service.Task{{ID: 1, TeamID: 10, Title: "Task"}}
			teams.EXPECT().MemberRole(mock.Anything, int64(10), int64(2)).Return(service.RoleMember, nil).Once()
			tasks.EXPECT().ListVersion(mock.Anything, int64(10)).Return(int64(7), nil).Once()
			if scenario == "hit" {
				cache.EXPECT().Get(mock.Anything, filter, int64(7)).Return(values, true, nil).Once()
			} else {
				var readErr, writeErr error
				if scenario == "read failure" {
					readErr = errors.New("redis unavailable")
				}
				if scenario == "write failure" {
					writeErr = errors.New("redis unavailable")
				}
				cache.EXPECT().Get(mock.Anything, filter, int64(7)).Return(nil, false, readErr).Once()
				tasks.EXPECT().List(mock.Anything, filter).Return(values, nil).Once()
				if readErr == nil {
					cache.EXPECT().Put(mock.Anything, filter, int64(7), values).Return(writeErr).Once()
				}
			}
			result, err := service.NewTasks(tasks, teams, tx, cache).List(context.Background(), 2, filter)
			require.NoError(t, err)
			require.Equal(t, values, result)
		})
	}
}
