package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"secunda/internal/service"
)

type TaskService interface {
	Create(context.Context, int64, service.CreateTaskInput) (service.Task, error)
	List(context.Context, int64, service.TaskFilter) ([]service.Task, error)
	Update(context.Context, int64, int64, service.UpdateTaskInput) (service.Task, error)
	History(context.Context, int64, int64) ([]service.TaskHistory, error)
	AddComment(context.Context, int64, int64, service.CreateCommentInput) (service.Comment, error)
	Comments(context.Context, int64, int64) ([]service.Comment, error)
}

type Tasks struct {
	service TaskService
	logger  *slog.Logger
}

func NewTasks(service TaskService, logger *slog.Logger) *Tasks {
	return &Tasks{service: service, logger: logger}
}

// Create godoc
// @Summary Create a task and its initial history
// @Tags tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body CreateTaskRequest true "Task"
// @Success 201 {object} TaskResponse
// @Failure 400,401,404,413,415,500 {object} ErrorResponse
// @Router /tasks [post]
func (h *Tasks) Create(w http.ResponseWriter, r *http.Request) {
	var request CreateTaskRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	task, err := h.service.Create(r.Context(), actorID(r.Context()), request.toService())
	if err != nil {
		serviceError(h.logger, w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, taskResponse(task))
}

// List godoc
// @Summary List tasks of a team
// @Tags tasks
// @Security BearerAuth
// @Produce json
// @Param team_id query int true "Team ID" minimum(1)
// @Param status query string false "Status" Enums(todo,in_progress,done)
// @Param assignee_id query int false "Assignee ID" minimum(1)
// @Param limit query int false "Page size" default(20) minimum(1) maximum(100)
// @Param offset query int false "Offset" default(0) minimum(0)
// @Success 200 {object} TasksResponse
// @Failure 400,401,404,500 {object} ErrorResponse
// @Router /tasks [get]
func (h *Tasks) List(w http.ResponseWriter, r *http.Request) {
	request, err := parseTaskListRequest(r.URL.Query())
	if err != nil {
		serviceError(h.logger, w, r, err)
		return
	}
	tasks, err := h.service.List(r.Context(), actorID(r.Context()), request.toService())
	if err != nil {
		serviceError(h.logger, w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, tasksResponse(tasks, request))
}

// Update godoc
// @Summary Update a task with optimistic concurrency control
// @Description Version is required. Omitted fields are unchanged. Null assignee_id removes the assignee. An assignee who is not the creator, owner or admin may only change status. Stale versions return 409.
// @Tags tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "Task ID" minimum(1)
// @Param request body UpdateTaskRequest true "Changes and current version"
// @Success 200 {object} TaskResponse
// @Failure 400,401,403,404,409,413,415,500 {object} ErrorResponse
// @Router /tasks/{id} [put]
func (h *Tasks) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseTaskID(w, r)
	if !ok {
		return
	}
	var request UpdateTaskRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	input, err := request.toService()
	if err != nil {
		serviceError(h.logger, w, r, err)
		return
	}
	task, err := h.service.Update(r.Context(), actorID(r.Context()), id, input)
	if err != nil {
		serviceError(h.logger, w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, taskResponse(task))
}

// History godoc
// @Summary Get task history in chronological order
// @Tags tasks
// @Security BearerAuth
// @Produce json
// @Param id path int true "Task ID" minimum(1)
// @Success 200 {object} HistoryResponse
// @Failure 400,401,404,500 {object} ErrorResponse
// @Router /tasks/{id}/history [get]
func (h *Tasks) History(w http.ResponseWriter, r *http.Request) {
	id, ok := parseTaskID(w, r)
	if !ok {
		return
	}
	history, err := h.service.History(r.Context(), actorID(r.Context()), id)
	if err != nil {
		serviceError(h.logger, w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, historyResponse(history))
}

// AddComment godoc
// @Summary Add a task comment
// @Tags tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "Task ID" minimum(1)
// @Param request body CreateCommentRequest true "Comment"
// @Success 201 {object} CommentResponse
// @Failure 400,401,404,413,415,500 {object} ErrorResponse
// @Router /tasks/{id}/comments [post]
func (h *Tasks) AddComment(w http.ResponseWriter, r *http.Request) {
	id, ok := parseTaskID(w, r)
	if !ok {
		return
	}
	var request CreateCommentRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	comment, err := h.service.AddComment(r.Context(), actorID(r.Context()), id, request.toService())
	if err != nil {
		serviceError(h.logger, w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, commentResponse(comment))
}

// Comments godoc
// @Summary Get task comments in chronological order
// @Tags tasks
// @Security BearerAuth
// @Produce json
// @Param id path int true "Task ID" minimum(1)
// @Success 200 {object} CommentsResponse
// @Failure 400,401,404,500 {object} ErrorResponse
// @Router /tasks/{id}/comments [get]
func (h *Tasks) Comments(w http.ResponseWriter, r *http.Request) {
	id, ok := parseTaskID(w, r)
	if !ok {
		return
	}
	comments, err := h.service.Comments(r.Context(), actorID(r.Context()), id)
	if err != nil {
		serviceError(h.logger, w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, commentsResponse(comments))
}

func parseTaskID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "task id must be a positive integer")
		return 0, false
	}
	return id, true
}
