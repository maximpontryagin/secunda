package handler

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"secunda/internal/service"
)

type CreateTaskRequest struct {
	TeamID      int64  `json:"team_id" validate:"required" minimum:"1" example:"1"`
	Title       string `json:"title" validate:"required" minLength:"1" maxLength:"255" example:"Implement login"`
	Description string `json:"description" example:"Add JWT authentication"`
	Status      string `json:"status" enums:"todo,in_progress,done" default:"todo"`
	AssigneeID  *int64 `json:"assignee_id" minimum:"1" extensions:"x-nullable" example:"2"`
}

func (r CreateTaskRequest) toService() service.CreateTaskInput {
	return service.CreateTaskInput{TeamID: r.TeamID, Title: r.Title, Description: r.Description, Status: service.TaskStatus(r.Status), AssigneeID: r.AssigneeID}
}

type UpdateTaskRequest struct {
	Version     int64           `json:"version" validate:"required" minimum:"1" example:"1"`
	Title       *string         `json:"title" minLength:"1" maxLength:"255" example:"Implement login"`
	Description *string         `json:"description" example:"Add JWT authentication"`
	Status      *string         `json:"status" enums:"todo,in_progress,done"`
	AssigneeID  json.RawMessage `json:"assignee_id" swaggertype:"integer" minimum:"1" extensions:"x-nullable" example:"2"`
}

func (r UpdateTaskRequest) toService() (service.UpdateTaskInput, error) {
	value := service.UpdateTaskInput{Version: r.Version, Title: r.Title, Description: r.Description}
	if r.Status != nil {
		status := service.TaskStatus(*r.Status)
		value.Status = &status
	}
	if len(r.AssigneeID) > 0 {
		value.SetAssignee = true
		if err := json.Unmarshal(r.AssigneeID, &value.AssigneeID); err != nil {
			return service.UpdateTaskInput{}, fmt.Errorf("%w: assignee_id must be an integer or null", service.ErrInvalid)
		}
	}
	return value, nil
}

type TaskListRequest struct {
	TeamID     int64
	Status     string
	AssigneeID *int64
	Limit      int
	Offset     int
}

func parseTaskListRequest(query url.Values) (TaskListRequest, error) {
	value := TaskListRequest{Status: query.Get("status"), Limit: 20}
	var err error
	value.TeamID, err = strconv.ParseInt(query.Get("team_id"), 10, 64)
	if err != nil || value.TeamID <= 0 {
		return TaskListRequest{}, fmt.Errorf("%w: team_id is required and must be positive", service.ErrInvalid)
	}
	if query.Has("assignee_id") {
		id, err := strconv.ParseInt(query.Get("assignee_id"), 10, 64)
		if err != nil || id <= 0 {
			return TaskListRequest{}, fmt.Errorf("%w: assignee_id must be positive", service.ErrInvalid)
		}
		value.AssigneeID = &id
	}
	if query.Has("limit") {
		value.Limit, err = strconv.Atoi(query.Get("limit"))
		if err != nil {
			return TaskListRequest{}, fmt.Errorf("%w: limit must be an integer", service.ErrInvalid)
		}
	}
	if query.Has("offset") {
		value.Offset, err = strconv.Atoi(query.Get("offset"))
		if err != nil {
			return TaskListRequest{}, fmt.Errorf("%w: offset must be an integer", service.ErrInvalid)
		}
	}
	return value, nil
}

func (r TaskListRequest) toService() service.TaskFilter {
	return service.TaskFilter{TeamID: r.TeamID, Status: service.TaskStatus(r.Status), AssigneeID: r.AssigneeID, Limit: r.Limit, Offset: r.Offset}
}

type TaskResponse struct {
	ID          int64      `json:"id" example:"1"`
	TeamID      int64      `json:"team_id" example:"1"`
	Title       string     `json:"title" example:"Implement login"`
	Description string     `json:"description"`
	Status      string     `json:"status" enums:"todo,in_progress,done"`
	CreatedBy   int64      `json:"created_by" example:"1"`
	AssigneeID  *int64     `json:"assignee_id" extensions:"x-nullable" example:"2"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ClosedAt    *time.Time `json:"closed_at" extensions:"x-nullable"`
	Version     int64      `json:"version" example:"1"`
}

func taskResponse(value service.Task) TaskResponse {
	return TaskResponse{ID: value.ID, TeamID: value.TeamID, Title: value.Title, Description: value.Description, Status: string(value.Status),
		CreatedBy: value.CreatedBy, AssigneeID: value.AssigneeID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, ClosedAt: value.ClosedAt, Version: value.Version}
}

type TasksResponse struct {
	Items  []TaskResponse `json:"items"`
	Limit  int            `json:"limit" example:"20"`
	Offset int            `json:"offset" example:"0"`
}

func tasksResponse(values []service.Task, request TaskListRequest) TasksResponse {
	items := make([]TaskResponse, 0, len(values))
	for _, value := range values {
		items = append(items, taskResponse(value))
	}
	return TasksResponse{Items: items, Limit: request.Limit, Offset: request.Offset}
}

type FieldChangeResponse struct {
	Old any `json:"old"`
	New any `json:"new"`
}

type TaskHistoryResponse struct {
	ID        int64                          `json:"id"`
	TaskID    int64                          `json:"task_id"`
	ChangedBy int64                          `json:"changed_by"`
	Changes   map[string]FieldChangeResponse `json:"changes"`
	CreatedAt time.Time                      `json:"created_at"`
}

type HistoryResponse struct {
	Items []TaskHistoryResponse `json:"items"`
}

func historyResponse(values []service.TaskHistory) HistoryResponse {
	items := make([]TaskHistoryResponse, 0, len(values))
	for _, value := range values {
		changes := make(map[string]FieldChangeResponse, len(value.Changes))
		for field, change := range value.Changes {
			changes[field] = FieldChangeResponse{Old: change.Old, New: change.New}
		}
		items = append(items, TaskHistoryResponse{ID: value.ID, TaskID: value.TaskID, ChangedBy: value.ChangedBy, Changes: changes, CreatedAt: value.CreatedAt})
	}
	return HistoryResponse{Items: items}
}

type CreateCommentRequest struct {
	Content string `json:"content" validate:"required" minLength:"1" example:"Ready for review"`
}

func (r CreateCommentRequest) toService() service.CreateCommentInput {
	return service.CreateCommentInput{Content: r.Content}
}

type CommentResponse struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	UserID    int64     `json:"user_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func commentResponse(value service.Comment) CommentResponse {
	return CommentResponse{ID: value.ID, TaskID: value.TaskID, UserID: value.UserID, Content: value.Content, CreatedAt: value.CreatedAt}
}

type CommentsResponse struct {
	Items []CommentResponse `json:"items"`
}

func commentsResponse(values []service.Comment) CommentsResponse {
	items := make([]CommentResponse, 0, len(values))
	for _, value := range values {
		items = append(items, commentResponse(value))
	}
	return CommentsResponse{Items: items}
}
