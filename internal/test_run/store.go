package testrun

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
}

func NewStore(connPool *pgxpool.Pool) *Store {
	return &Store{connPool}
}

func (s *Store) Create(ctx context.Context, input CreateInput) (TestRun, error) {
	id := uuid.New()
	createdAt := time.Now().Unix()
	status := StatusPending
	_, err := s.db.Exec(
		ctx,
		`INSERT INTO "Test_runs" (
			id,
			deploy_base_url,
			telegram_username,
			telegram_user_id,
			github_username,
			github_repository,
			project_language,
			project_name,
			created_at,
			completed_at,
			status,
			error,
			report,
			total,
			passed,
			failed,
			skipped
		)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
		id,
		input.DeployBaseURL,
		input.TelegramUsername,
		input.TelegramUserID,
		input.GithubUsername,
		input.GithubRepository,
		input.ProjectLanguage,
		input.ProjectName,
		createdAt,
		nil,
		status,
		nil,
		nil,
		input.Progress.Total,
		input.Progress.Passed,
		input.Progress.Failed,
		input.Progress.Skipped,
	)
	if err != nil {
		return TestRun{}, fmt.Errorf("store.Create: %w", err)
	}
	return TestRun{
		id,
		input.DeployBaseURL,
		input.TelegramUsername,
		input.TelegramUserID,
		input.GithubUsername,
		input.GithubRepository,
		input.ProjectLanguage,
		input.ProjectName,
		createdAt,
		nil,
		status,
		nil,
		nil,
		input.Progress,
	}, nil
}

func (s *Store) FindByID(ctx context.Context, id uuid.UUID) (TestRun, error) {
	rows, err := s.db.Query(
		ctx,
		`SELECT
			id,
			deploy_base_url,
			telegram_username,
			telegram_user_id,
			github_username,
			github_repository,
			project_language,
			project_name,
			created_at,
			completed_at,
			status,
			error,
			report,
			total,
			passed,
			failed,
			skipped
		FROM "Test_runs" WHERE id = $1`,
		id,
	)
	if err != nil {
		return TestRun{}, fmt.Errorf("store.FindByID: %w", err)
	}
	testRun, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[TestRun])

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TestRun{}, ErrNotFound
		}
		return TestRun{}, fmt.Errorf("store.FindByID: %w", err)
	}

	return testRun, nil
}

func (s *Store) List(ctx context.Context, filters filters) ([]TestRun, error) {
	query := `
		SELECT 
			id, 
			deploy_base_url, 
			telegram_username, 
			telegram_user_id,
			github_username, 
			github_repository, 
			project_language, 
			project_name,
			created_at, 
			completed_at, 
			status, 
			error,
			report,
			total,
			passed,
			failed,
			skipped
	        FROM "Test_runs"`

	var conditions []string
	var args []any
	i := 1

	if filters.DeployBaseURL != nil {
		conditions = append(conditions, fmt.Sprintf("deploy_base_url = $%d", i))
		args = append(args, *filters.DeployBaseURL)
		i++
	}
	if filters.TelegramUsername != nil {
		conditions = append(conditions, fmt.Sprintf("telegram_username = $%d", i))
		args = append(args, *filters.TelegramUsername)
		i++
	}
	if filters.TelegramUserID != nil {
		conditions = append(conditions, fmt.Sprintf("telegram_user_id = $%d", i))
		args = append(args, *filters.TelegramUserID)
		i++
	}
	if filters.GithubUsername != nil {
		conditions = append(conditions, fmt.Sprintf("github_username = $%d", i))
		args = append(args, *filters.GithubUsername)
		i++
	}
	if filters.GithubRepository != nil {
		conditions = append(conditions, fmt.Sprintf("github_repository = $%d", i))
		args = append(args, *filters.GithubRepository)
		i++
	}
	if filters.ProjectLanguage != nil {
		conditions = append(conditions, fmt.Sprintf("project_language = $%d", i))
		args = append(args, *filters.ProjectLanguage)
		i++
	}
	if filters.ProjectName != nil {
		conditions = append(conditions, fmt.Sprintf("project_name = $%d", i))
		args = append(args, *filters.ProjectName)
		i++
	}
	if filters.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", i))
		args = append(args, *filters.Status)
		i++
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store.List: %w", err)
	}
	runs, err := pgx.CollectRows(rows, pgx.RowToStructByName[TestRun])
	if err != nil {
		return nil, fmt.Errorf("store.List: %w", err)
	}

	return runs, nil
}

func (s *Store) Update(ctx context.Context, id uuid.UUID, input updateParams) error {
	_, err := s.db.Exec(
		ctx,
		`UPDATE "Test_runs"
		SET
			completed_at = $1,
			status = $2,
			error = $3,
			report = $4,
			passed = $5,
			failed = $6,
			skipped = $7
		WHERE id = $8`,
		input.CompletedAt,
		input.Status,
		input.Error,
		input.Report,
		input.Progress.Passed,
		input.Progress.Failed,
		input.Progress.Skipped,
		id,
	)
	if err != nil {
		return fmt.Errorf("store.Update: %w", err)
	}
	return nil
}
