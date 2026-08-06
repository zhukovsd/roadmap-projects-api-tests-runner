ALTER TABLE "Test_runs"
ALTER COLUMN telegram_username DROP NOT NULL,
ALTER COLUMN telegram_user_id DROP NOT NULL,
ALTER COLUMN github_username DROP NOT NULL,
ALTER COLUMN github_repository DROP NOT NULL,
ALTER COLUMN project_language DROP NOT NULL;
