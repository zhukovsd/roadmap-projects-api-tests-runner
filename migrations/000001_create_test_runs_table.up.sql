CREATE TABLE IF NOT EXISTS "Test_runs" (
    id UUID PRIMARY KEY,
    deploy_base_url VARCHAR NOT NULL,
    telegram_username VARCHAR NOT NULL,
    telegram_user_id BIGINT NOT NULL,
    github_username VARCHAR NOT NULL,
    github_repository VARCHAR NOT NULL,
    project_language VARCHAR NOT NULL,
    project_name VARCHAR NOT NULL,
    created_at BIGINT NOT NULL,
    completed_at BIGINT,
    status VARCHAR NOT NULL,
    error VARCHAR
);
