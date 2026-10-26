package service

import (
	"context"
	"time"
)

type TaskCounts struct {
	Todo       int64
	InProgress int64
	Done       int64
}

type Performer struct {
	UserID      int64
	Name        string
	ClosedTasks int64
}

type Statistics struct {
	TeamID              int64
	TasksByStatus       TaskCounts
	TopAssignees        []Performer
	AverageCloseSeconds *float64
	CommentsCount       int64
}

type StatisticsRepository interface {
	Get(context.Context, int64, time.Time) (Statistics, error)
}

type Reports struct {
	statistics StatisticsRepository
	teams      TeamRepository
}

func NewReports(statistics StatisticsRepository, teams TeamRepository) *Reports {
	return &Reports{statistics: statistics, teams: teams}
}

func (s *Reports) Get(ctx context.Context, actorID, teamID int64) (Statistics, error) {
	if actorID <= 0 {
		return Statistics{}, ErrUnauthorized
	}
	if teamID <= 0 {
		return Statistics{}, invalid("team id must be positive")
	}
	role, err := s.teams.MemberRole(ctx, teamID, actorID)
	if err != nil {
		return Statistics{}, err
	}
	if role != RoleOwner && role != RoleAdmin {
		return Statistics{}, ErrForbidden
	}
	return s.statistics.Get(ctx, teamID, time.Now().UTC())
}
