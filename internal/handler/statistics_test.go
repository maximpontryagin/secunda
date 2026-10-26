package handler_test

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"secunda/internal/handler"
	mocks "secunda/internal/mocks/handler"
	"secunda/internal/service"
)

func TestStatisticsRouteAndEmptyResponse(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	statistics := mocks.NewStatisticsService(t)
	verifier := mocks.NewTokenVerifier(t)
	verifier.EXPECT().Verify("token").Return(int64(2), nil).Once()
	statistics.EXPECT().Get(mock.Anything, int64(2), int64(10)).Return(service.Statistics{TeamID: 10}, nil).Once()
	router := handler.NewRouter(handler.NewAuth(mocks.NewAuthService(t), logger), handler.NewTeams(mocks.NewTeamService(t), logger),
		handler.NewTasks(mocks.NewTaskService(t), logger), handler.NewStatistics(statistics, logger), verifier)
	request := httptest.NewRequest("GET", "/api/v1/teams/10/stats", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, 200, response.Code)
	require.JSONEq(t, `{"team_id":10,"tasks_by_status":{"todo":0,"in_progress":0,"done":0},"top_assignees":[],"average_close_seconds":null,"comments_count":0}`, response.Body.String())
}
