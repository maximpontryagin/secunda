package service

import "strings"

func initialChanges(task Task) map[string]FieldChange {
	values := map[string]any{
		"id": task.ID, "team_id": task.TeamID, "title": task.Title, "description": task.Description,
		"status": task.Status, "created_by": task.CreatedBy, "assignee_id": task.AssigneeID,
		"created_at": task.CreatedAt, "updated_at": task.UpdatedAt, "closed_at": task.ClosedAt, "version": task.Version,
	}
	changes := make(map[string]FieldChange, len(values))
	for name, value := range values {
		changes[name] = FieldChange{Old: nil, New: value}
	}
	return changes
}

func applyTaskUpdate(current Task, input UpdateTaskInput) (Task, map[string]FieldChange, error) {
	next := current
	changes := make(map[string]FieldChange)
	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		if !validName(title) {
			return Task{}, nil, invalid("title must contain between 1 and 255 characters")
		}
		if title != current.Title {
			next.Title = title
			changes["title"] = FieldChange{Old: current.Title, New: title}
		}
	}
	if input.Description != nil {
		if !validText(*input.Description) {
			return Task{}, nil, invalid("description must contain at most 65535 bytes of UTF-8 text")
		}
		if *input.Description != current.Description {
			next.Description = *input.Description
			changes["description"] = FieldChange{Old: current.Description, New: next.Description}
		}
	}
	if input.Status != nil {
		if !validStatus(*input.Status) {
			return Task{}, nil, invalid("status must be todo, in_progress or done")
		}
		if *input.Status != current.Status {
			next.Status = *input.Status
			changes["status"] = FieldChange{Old: current.Status, New: next.Status}
		}
	}
	if input.SetAssignee && !sameAssignee(current.AssigneeID, input.AssigneeID) {
		next.AssigneeID = input.AssigneeID
		changes["assignee_id"] = FieldChange{Old: current.AssigneeID, New: next.AssigneeID}
	}
	return next, changes, nil
}

func sameAssignee(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
