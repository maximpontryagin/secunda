-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE users (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(254) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    UNIQUE KEY uq_users_email (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE teams (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_by BIGINT NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT fk_teams_creator FOREIGN KEY (created_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE team_members (
    team_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    role VARCHAR(16) NOT NULL,
    PRIMARY KEY (team_id, user_id),
    KEY idx_members_user (user_id, team_id),
    CONSTRAINT chk_members_role CHECK (role IN ('owner', 'admin', 'member')),
    CONSTRAINT fk_members_team FOREIGN KEY (team_id) REFERENCES teams(id),
    CONSTRAINT fk_members_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE tasks (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    team_id BIGINT NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'todo',
    created_by BIGINT NOT NULL,
    assignee_id BIGINT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    closed_at DATETIME(6) NULL,
    version BIGINT NOT NULL DEFAULT 1,
    KEY idx_tasks_team_id (team_id, id),
    KEY idx_tasks_team_status (team_id, status, id),
    KEY idx_tasks_team_assignee (team_id, assignee_id, id),
    KEY idx_tasks_team_closed (team_id, status, closed_at, assignee_id),
    CONSTRAINT chk_tasks_status CHECK (status IN ('todo', 'in_progress', 'done')),
    CONSTRAINT chk_tasks_version CHECK (version > 0),
    CONSTRAINT chk_tasks_closed CHECK ((status = 'done' AND closed_at IS NOT NULL) OR (status <> 'done' AND closed_at IS NULL)),
    CONSTRAINT fk_tasks_team FOREIGN KEY (team_id) REFERENCES teams(id),
    CONSTRAINT fk_tasks_creator FOREIGN KEY (team_id, created_by) REFERENCES team_members(team_id, user_id),
    CONSTRAINT fk_tasks_assignee FOREIGN KEY (team_id, assignee_id) REFERENCES team_members(team_id, user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE task_history (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    task_id BIGINT NOT NULL,
    changed_by BIGINT NOT NULL,
    changes JSON NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    KEY idx_history_task (task_id, id),
    CONSTRAINT fk_history_task FOREIGN KEY (task_id) REFERENCES tasks(id),
    CONSTRAINT fk_history_user FOREIGN KEY (changed_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE task_comments (
    id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
    task_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    KEY idx_comments_task (task_id, id),
    CONSTRAINT fk_comments_task FOREIGN KEY (task_id) REFERENCES tasks(id),
    CONSTRAINT fk_comments_user FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE task_comments;
DROP TABLE task_history;
DROP TABLE tasks;
DROP TABLE team_members;
DROP TABLE teams;
DROP TABLE users;
