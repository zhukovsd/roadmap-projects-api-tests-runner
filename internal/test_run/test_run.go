// Package testrun provides types and operations for managing test runs.
package testrun

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

type TestRunStatus string
type TestRunProjectName string

const (
	StatusPending   TestRunStatus = "PENDING"
	StatusRunning   TestRunStatus = "RUNNING"
	StatusFailed    TestRunStatus = "FAILED"
	StatusCompleted TestRunStatus = "COMPLETED"
)

const (
	ProjectNameCurrencyExchange TestRunProjectName = "CURRENCY_EXCHANGE"
	ProjectNameTennisScoreboard TestRunProjectName = "TENNIS_SCOREBOARD"
	ProjectNameCloudFileStorage TestRunProjectName = "CLOUD_FILE_STORAGE"
)

type TestRun struct {
	ID               uuid.UUID          `db:"id" json:"id"`
	DeployBaseURL    string             `db:"deploy_base_url" json:"deploy_base_url"`
	TelegramUsername string             `db:"telegram_username" json:"telegram_username"`
	TelegramUserID   int64              `db:"telegram_user_id" json:"telegram_user_id"`
	GithubUsername   string             `db:"github_username" json:"github_username"`
	GithubRepository string             `db:"github_repository" json:"github_repository"`
	ProjectLanguage  string             `db:"project_language" json:"project_language"`
	ProjectName      TestRunProjectName `db:"project_name" json:"project_name"`
	CreatedAt        int64              `db:"created_at" json:"created_at"`
	CompletedAt      *int64             `db:"completed_at" json:"completed_at"`
	Status           TestRunStatus      `db:"status" json:"status"`
	Error            *string            `db:"error" json:"error"`
	Report           *json.RawMessage   `db:"report" json:"report"`
}

type CreateInput struct {
	DeployBaseURL    string             `json:"deploy_base_url"`
	TelegramUsername string             `json:"telegram_username"`
	TelegramUserID   int64              `json:"telegram_user_id"`
	GithubUsername   string             `json:"github_username"`
	GithubRepository string             `json:"github_repository"`
	ProjectLanguage  string             `json:"project_language"`
	ProjectName      TestRunProjectName `json:"project_name"`
}

type updateParams struct {
	CompletedAt *int64
	Status      TestRunStatus
	Error       *string
	Report      *json.RawMessage
}

type filters struct {
	DeployBaseURL    *string
	TelegramUsername *string
	TelegramUserID   *int64
	GithubUsername   *string
	GithubRepository *string
	ProjectLanguage  *string
	ProjectName      *string
	Status           *TestRunStatus
}

func (i CreateInput) Validate() []error {
	var errors []error

	if i.DeployBaseURL == "" {
		errors = append(errors, fmt.Errorf("missing \"deploy_base_url\": %w", ErrMissingField))
	}
	if i.TelegramUsername == "" {
		errors = append(errors, fmt.Errorf("missing \"telegram_username\": %w", ErrMissingField))
	}
	if i.TelegramUserID == 0 {
		errors = append(errors, fmt.Errorf("missing \"telegram_user_id\": %w", ErrMissingField))
	}
	if i.GithubUsername == "" {
		errors = append(errors, fmt.Errorf("missing \"github_username\": %w", ErrMissingField))
	}
	if i.GithubRepository == "" {
		errors = append(errors, fmt.Errorf("missing \"github_repository\": %w", ErrMissingField))
	}
	if i.ProjectLanguage == "" {
		errors = append(errors, fmt.Errorf("missing \"project_language\": %w", ErrMissingField))
	}
	if i.ProjectName == "" {
		errors = append(errors, fmt.Errorf("missing \"project_name\": %w", ErrMissingField))
	}
	if err := i.ProjectName.Validate(); err != nil {
		errors = append(errors, err)
	}

	return errors
}

func (s TestRunStatus) Validate() error {
	switch s {
	case StatusPending, StatusRunning, StatusFailed, StatusCompleted:
		return nil
	default:
		return fmt.Errorf("invalid test run status value %q: %w", s, ErrInvalidField)
	}
}

func (p TestRunProjectName) Validate() error {
	switch p {
	case ProjectNameCurrencyExchange, ProjectNameTennisScoreboard, ProjectNameCloudFileStorage:
		return nil
	default:
		return fmt.Errorf("invalid test run project name value %q: %w", p, ErrInvalidField)
	}
}
