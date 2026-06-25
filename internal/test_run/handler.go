package testrun

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/zhukovsd/roadmap-projects-api-test-runner/internal/logger"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service}
}

func (h *Handler) Create(resp http.ResponseWriter, req *http.Request) {
	var input CreateInput

	if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
		logger.Error("Failed to parse request", "error", err)
		http.Error(resp, "Bad request", http.StatusBadRequest)
		return
	}

	if errs := input.Validate(); len(errs) > 0 {
		logger.Error("Invalid create test run request", "errors", errors.Join(errs...), "input", input)
		http.Error(resp, "Bad request", http.StatusBadRequest)
		return
	}

	testRun, err := h.service.Create(req.Context(), input)

	if err != nil {
		logger.Error("Failed to create test run", "error", err, "input", input)
		http.Error(resp, "Internal server error", http.StatusInternalServerError)
		return
	}

	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(resp).Encode(testRun); err != nil {
		logger.Error("Failed to encode test run", "error", err, "testRun", testRun)
		http.Error(resp, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) FindByID(resp http.ResponseWriter, req *http.Request) {
	reqID := req.PathValue("id")
	id, err := uuid.Parse(reqID)

	if err != nil {
		logger.Error("Failed to parse request id", "error", err, "id", reqID)
		http.Error(resp, "Bad request", http.StatusBadRequest)
		return
	}

	testRun, err := h.service.FindByID(req.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(resp, "Not found", http.StatusNotFound)
			return
		}
		logger.Error("Failed to find test run", "error", err, "id", id)
		http.Error(resp, "Internal server error", http.StatusInternalServerError)
		return
	}

	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(resp).Encode(testRun); err != nil {
		logger.Error("Failed to encode test run", "error", err, "testRun", testRun)
		http.Error(resp, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) List(resp http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()

	tgIDParam := emptyToNil(q.Get("telegramUserId"))
	var tgID *int64
	if tgIDParam != nil {
		id, err := strconv.ParseInt(*tgIDParam, 10, 64)
		if err != nil {
			logger.Error("Failed to convert string to int64", "error", err, "id", *tgIDParam)
			http.Error(resp, "Bad request", http.StatusBadRequest)
			return
		}
		tgID = &id
	}

	statusParam := emptyToNil(q.Get("status"))
	var status *TestRunStatus
	if statusParam != nil {
		s := TestRunStatus(*statusParam)

		if err := s.Validate(); err != nil {
			logger.Error("Failed to parse test run status", "error", err, "status", *statusParam)
			http.Error(resp, "Bad request", http.StatusBadRequest)
			return
		}

		status = &s
	}

	filters := Filters{
		DeployBaseURL:    emptyToNil(q.Get("deployBaseUrl")),
		TelegramUsername: emptyToNil(q.Get("telegramUsername")),
		TelegramUserID:   tgID,
		GithubUsername:   emptyToNil(q.Get("githubUsername")),
		GithubRepository: emptyToNil(q.Get("githubRepository")),
		ProjectLanguage:  emptyToNil(q.Get("projectLanguage")),
		ProjectName:      emptyToNil(q.Get("projectName")),
		Status:           status,
	}

	testRuns, err := h.service.List(req.Context(), filters)
	if err != nil {
		logger.Error("Failed to find test runs", "error", err, "filters", filters)
		http.Error(resp, "Internal server error", http.StatusInternalServerError)
		return
	}

	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(resp).Encode(testRuns); err != nil {
		logger.Error("Failed to encode test runs", "error", err, "testRuns", testRuns)
		http.Error(resp, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func emptyToNil(s string) *string {
	if len(s) == 0 || s == "" {
		return nil
	}
	return &s
}
