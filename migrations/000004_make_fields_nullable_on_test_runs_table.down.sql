ALTER TABLE "Test_runs"
ALTER COLUMN telegram_username SET NOT NULL,
ALTER COLUMN telegram_user_id SET NOT NULL,
ALTER COLUMN github_username SET NOT NULL,
ALTER COLUMN github_repository SET NOT NULL,
ALTER COLUMN project_language SET NOT NULL;
