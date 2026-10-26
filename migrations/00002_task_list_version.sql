-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE teams ADD COLUMN task_list_version BIGINT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE teams DROP COLUMN task_list_version;
