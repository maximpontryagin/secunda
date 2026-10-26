package repository

import (
	"context"
	"database/sql"
	"time"

	"secunda/internal/platform/transaction"
	"secunda/internal/service"
)

type Statistics struct{ db *sql.DB }

func NewStatistics(db *sql.DB) *Statistics { return &Statistics{db: db} }

const statisticsQuery = `
WITH scoped_tasks AS (
    SELECT id, status, assignee_id, created_at, closed_at
    FROM tasks
    WHERE team_id = ?
), summary AS (
    SELECT COALESCE(SUM(status = 'todo'), 0) AS todo,
           COALESCE(SUM(status = 'in_progress'), 0) AS in_progress,
           COALESCE(SUM(status = 'done'), 0) AS done,
           AVG(CASE WHEN status = 'done'
               THEN TIMESTAMPDIFF(MICROSECOND, created_at, closed_at) / 1000000.0
               END) AS average_close_seconds
    FROM scoped_tasks
), top_assignees AS (
    SELECT assignee_id, COUNT(*) AS closed_tasks
    FROM scoped_tasks
    WHERE status = 'done'
      AND assignee_id IS NOT NULL
      AND closed_at >= DATE_SUB(?, INTERVAL 30 DAY)
      AND closed_at <= ?
    GROUP BY assignee_id
    ORDER BY closed_tasks DESC, assignee_id ASC
    LIMIT 3
), comment_totals AS (
    SELECT COUNT(*) AS comments_count
    FROM task_comments c
    JOIN scoped_tasks t ON t.id = c.task_id
)
SELECT s.todo, s.in_progress, s.done, s.average_close_seconds,
       c.comments_count, p.assignee_id, u.name, p.closed_tasks
FROM summary s
CROSS JOIN comment_totals c
LEFT JOIN top_assignees p ON TRUE
LEFT JOIN users u ON u.id = p.assignee_id
ORDER BY p.closed_tasks DESC, p.assignee_id ASC`

type statisticsRow struct {
	Todo                int64
	InProgress          int64
	Done                int64
	AverageCloseSeconds sql.NullFloat64
	CommentsCount       int64
	UserID              sql.NullInt64
	UserName            sql.NullString
	ClosedTasks         sql.NullInt64
}

func (r statisticsRow) toService(teamID int64) service.Statistics {
	value := service.Statistics{TeamID: teamID,
		TasksByStatus: service.TaskCounts{Todo: r.Todo, InProgress: r.InProgress, Done: r.Done},
		CommentsCount: r.CommentsCount, TopAssignees: make([]service.Performer, 0, 3)}
	if r.AverageCloseSeconds.Valid {
		value.AverageCloseSeconds = &r.AverageCloseSeconds.Float64
	}
	return value
}

func (r statisticsRow) performer() service.Performer {
	return service.Performer{UserID: r.UserID.Int64, Name: r.UserName.String, ClosedTasks: r.ClosedTasks.Int64}
}

func (r *Statistics) Get(ctx context.Context, teamID int64, asOf time.Time) (service.Statistics, error) {
	rows, err := transaction.Connection(ctx, r.db).QueryContext(ctx, statisticsQuery, teamID, asOf, asOf)
	if err != nil {
		return service.Statistics{}, databaseError("query team statistics", err)
	}
	defer rows.Close()
	var result service.Statistics
	for rows.Next() {
		var row statisticsRow
		if err := rows.Scan(&row.Todo, &row.InProgress, &row.Done, &row.AverageCloseSeconds, &row.CommentsCount, &row.UserID, &row.UserName, &row.ClosedTasks); err != nil {
			return service.Statistics{}, databaseError("scan team statistics", err)
		}
		if result.TeamID == 0 {
			result = row.toService(teamID)
		}
		if row.UserID.Valid {
			result.TopAssignees = append(result.TopAssignees, row.performer())
		}
	}
	if err := rows.Err(); err != nil {
		return service.Statistics{}, databaseError("iterate team statistics", err)
	}
	return result, nil
}

var _ service.StatisticsRepository = (*Statistics)(nil)
