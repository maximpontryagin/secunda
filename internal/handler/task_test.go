package handler_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"secunda/internal/handler"
	mocks "secunda/internal/mocks/handler"
	"secunda/internal/service"
)

func taskRouter(t *testing.T) (http.Handler, *mocks.TaskService) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	auth := mocks.NewAuthService(t)
	teams := mocks.NewTeamService(t)
	tasks := mocks.NewTaskService(t)
	verifier := mocks.NewTokenVerifier(t)
	verifier.EXPECT().Verify("token").Return(int64(7), nil).Once()
	return handler.NewRouter(handler.NewAuth(auth, logger), handler.NewTeams(teams, logger), handler.NewTasks(tasks, logger), handler.NewStatistics(mocks.NewStatisticsService(t), logger), verifier), tasks
}

func TestUpdateAssigneePresence(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		set        bool
		id         int64
	}{
		{name: "omitted", body: `{"version":1}`},
		{name: "null clears", body: `{"version":1,"assignee_id":null}`, set: true},
		{name: "id assigns", body: `{"version":1,"assignee_id":42}`, set: true, id: 42},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router, tasks := taskRouter(t)
			tasks.EXPECT().Update(mock.Anything, int64(7), int64(10), mock.MatchedBy(func(value service.UpdateTaskInput) bool {
				if value.SetAssignee != tc.set || value.Version != 1 {
					return false
				}
				if tc.id == 0 {
					return value.AssigneeID == nil
				}
				return value.AssigneeID != nil && *value.AssigneeID == tc.id
			})).Return(service.Task{ID: 10, Version: 2}, nil).Once()
			request := httptest.NewRequest("PUT", "/api/v1/tasks/10", strings.NewReader(tc.body))
			request.Header.Set("Authorization", "Bearer token")
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, 200, response.Code)
		})
	}
}

func TestTaskQueryRejectsInvalidNumbers(t *testing.T) {
	for _, query := range []string{"", "team_id=bad", "team_id=1&assignee_id=1.5", "team_id=1&limit=oops", "team_id=1&offset=oops"} {
		router, _ := taskRouter(t)
		request := httptest.NewRequest("GET", "/api/v1/tasks?"+query, nil)
		request.Header.Set("Authorization", "Bearer token")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, 400, response.Code, query)
	}
}

func TestTaskListDefaultPagination(t *testing.T) {
	router, tasks := taskRouter(t)
	tasks.EXPECT().List(mock.Anything, int64(7), service.TaskFilter{TeamID: 1, Limit: 20}).Return(nil, nil).Once()
	request := httptest.NewRequest("GET", "/api/v1/tasks?team_id=1", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, 200, response.Code)
	require.JSONEq(t, `{"items":[],"limit":20,"offset":0}`, response.Body.String())
}

func TestTaskVersionConflictResponse(t *testing.T) {
	router, tasks := taskRouter(t)
	tasks.EXPECT().Update(mock.Anything, int64(7), int64(10), mock.Anything).Return(service.Task{}, service.ErrVersionConflict).Once()
	request := httptest.NewRequest("PUT", "/api/v1/tasks/10", strings.NewReader(`{"version":1,"title":"New"}`))
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, 409, response.Code)
	require.Contains(t, response.Body.String(), `"code":"version_conflict"`)
}
