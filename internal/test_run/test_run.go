// Package testrun provides types and operations for managing test runs.
package testrun

import (
	"fmt"

	"github.com/google/uuid"
)

type TestRunStatus string

const (
	StatusPending   TestRunStatus = "PENDING"
	StatusRunning   TestRunStatus = "RUNNING"
	StatusFailed    TestRunStatus = "FAILED"
	StatusCompleted TestRunStatus = "COMPLETED"
)

type TestRun struct {
	ID               uuid.UUID     `db:"id" json:"id"`
	DeployBaseURL    string        `db:"deploy_base_url" json:"deploy_base_url"`
	TelegramUsername string        `db:"telegram_username" json:"telegram_username"`
	TelegramUserID   int64         `db:"telegram_user_id" json:"telegram_user_id"`
	GithubUsername   string        `db:"github_username" json:"github_username"`
	GithubRepository string        `db:"github_repository" json:"github_repository"`
	ProjectLanguage  string        `db:"project_language" json:"project_language"`
	ProjectName      string        `db:"project_name" json:"project_name"`
	CreatedAt        int64         `db:"created_at" json:"created_at"`
	CompletedAt      *int64        `db:"completed_at" json:"completed_at"`
	Status           TestRunStatus `db:"status" json:"status"`
	Error            *string       `db:"error" json:"error"`
}

type CreateInput struct {
	DeployBaseURL    string `json:"deploy_base_url"`
	TelegramUsername string `json:"telegram_username"`
	TelegramUserID   int64  `json:"telegram_user_id"`
	GithubUsername   string `json:"github_username"`
	GithubRepository string `json:"github_repository"`
	ProjectLanguage  string `json:"project_language"`
	ProjectName      string `json:"project_name"`
}

type Filters struct {
	DeployBaseURL    *string
	TelegramUsername *string
	TelegramUserID   *int64
	GithubUsername   *string
	GithubRepository *string
	ProjectLanguage  *string
	ProjectName      *string
	Status           *TestRunStatus
}

func ParseTestRunStatus(s string) (TestRunStatus, error) {
	switch TestRunStatus(s) {
	case StatusPending, StatusRunning, StatusFailed, StatusCompleted:
		return TestRunStatus(s), nil
	default:
		return "", fmt.Errorf("invalid test run status: %q", s)
	}
}
