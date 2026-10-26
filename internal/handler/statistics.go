package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"secunda/internal/service"
)

type StatisticsService interface {
	Get(context.Context, int64, int64) (service.Statistics, error)
}

type Statistics struct {
	service StatisticsService
	logger  *slog.Logger
}

func NewStatistics(service StatisticsService, logger *slog.Logger) *Statistics {
	return &Statistics{service: service, logger: logger}
}

type TaskCountsResponse struct {
	Todo       int64 `json:"todo" example:"3"`
	InProgress int64 `json:"in_progress" example:"2"`
	Done       int64 `json:"done" example:"5"`
}

type PerformerResponse struct {
	UserID      int64  `json:"user_id" example:"2"`
	Name        string `json:"name" example:"Alice"`
	ClosedTasks int64  `json:"closed_tasks" example:"5"`
}

type StatisticsResponse struct {
	TeamID              int64               `json:"team_id" example:"1"`
	TasksByStatus       TaskCountsResponse  `json:"tasks_by_status"`
	TopAssignees        []PerformerResponse `json:"top_assignees"`
	AverageCloseSeconds *float64            `json:"average_close_seconds" extensions:"x-nullable" example:"3600"`
	CommentsCount       int64               `json:"comments_count" example:"10"`
}

func statisticsResponse(value service.Statistics) StatisticsResponse {
	performers := make([]PerformerResponse, 0, len(value.TopAssignees))
	for _, performer := range value.TopAssignees {
		performers = append(performers, PerformerResponse{UserID: performer.UserID, Name: performer.Name, ClosedTasks: performer.ClosedTasks})
	}
	return StatisticsResponse{TeamID: value.TeamID,
		TasksByStatus: TaskCountsResponse{Todo: value.TasksByStatus.Todo, InProgress: value.TasksByStatus.InProgress, Done: value.TasksByStatus.Done},
		TopAssignees:  performers, AverageCloseSeconds: value.AverageCloseSeconds, CommentsCount: value.CommentsCount}
}

// Get godoc
// @Summary Get team statistics
// @Description Owner/admin only. Status counts and comments cover all team tasks. Average close time covers all currently done tasks, in seconds (null if none). Top 3 assignees cover tasks closed in the last 30 days, ordered by count descending then user ID ascending. Unassigned tasks are excluded from the top.
// @Tags teams
// @Security BearerAuth
// @Produce json
// @Param team_id path int true "Team ID" minimum(1)
// @Success 200 {object} StatisticsResponse
// @Failure 400,401,403,404,500 {object} ErrorResponse
// @Router /teams/{team_id}/stats [get]
func (h *Statistics) Get(w http.ResponseWriter, r *http.Request) {
	teamID, err := strconv.ParseInt(chi.URLParam(r, "team_id"), 10, 64)
	if err != nil || teamID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "team id must be a positive integer")
		return
	}
	report, err := h.service.Get(r.Context(), actorID(r.Context()), teamID)
	if err != nil {
		serviceError(h.logger, w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, statisticsResponse(report))
}
