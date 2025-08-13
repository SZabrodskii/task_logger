package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"task_api/internal/logger"
	"task_api/internal/repository"
	"task_api/internal/task"
	"task_api/pkg"
)

const (
	defaultLimit  = 10
	defaultOffset = 0
)

type TaskHandler struct {
	service task.Service
	log     logger.Logger
}

func NewTaskHandler(service task.Service, log logger.Logger) *TaskHandler {
	return &TaskHandler{
		service: service,
		log:     log,
	}
}

func (h *TaskHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/tasks", h.handleTasks)
	mux.HandleFunc("/tasks/", h.handleTaskByID)
}

func (h *TaskHandler) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getTasks(w, r)
	case http.MethodPost:
		h.createTask(w, r)
	default:
		pkg.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *TaskHandler) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		pkg.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/tasks/")
	if id == "" {
		pkg.WriteError(w, http.StatusBadRequest, "Invalid task ID")
		return
	}
	h.getTaskByID(w, r, id)
}

func (h *TaskHandler) getTasks(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context()).With("where", "handler")
	statusFilter := r.URL.Query().Get("status")

	limit, err := parseIntQueryParam(r, "limit", defaultLimit)
	if err != nil {
		log.Debug("handler: error parsing limit", "error", err)
		pkg.WriteError(w, http.StatusBadRequest, "Invalid limit")
		return
	}

	offset, err := parseIntQueryParam(r, "offset", defaultOffset)
	if err != nil {
		log.Debug("handler: error parsing offset", "error", err)
		pkg.WriteError(w, http.StatusBadRequest, "Invalid offset")
		return
	}

	tasks, err := h.service.GetAllTasks(r.Context(), statusFilter, limit, offset)
	if err != nil {
		log.Error("handler: error getting tasks", "error", err)
		pkg.WriteError(w, http.StatusInternalServerError, "error getting tasks")
		return
	}
	log.Info("handler: tasks retrieved from repository", "count", len(tasks))
	pkg.WriteJSON(w, http.StatusOK, tasks)
}

func (h *TaskHandler) getTaskByID(w http.ResponseWriter, r *http.Request, id string) {
	log := logger.FromContext(r.Context()).With("where", "handler")
	t, err := h.service.GetTaskByID(r.Context(), id)
	if err != nil {
		log.Error("handler: error getting task", "error", err)
		if errors.Is(err, repository.ErrNotFound) {
			pkg.WriteError(w, http.StatusNotFound, err.Error())
		} else {
			pkg.WriteError(w, http.StatusInternalServerError, "error getting task")
		}
		return
	}
	log.Info("handler: task retrieved from repository", "id", t.ID)
	pkg.WriteJSON(w, http.StatusOK, t)
}

func (h *TaskHandler) createTask(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context()).With("where", "handler")
	var reqBody struct {
		Title string `json:"title"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		log.Debug("handler: error decoding request body", "error", err)
		pkg.WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if reqBody.Title == "" {
		log.Debug("handler: title is required")
		pkg.WriteError(w, http.StatusBadRequest, "Title is required")
		return
	}

	createdTask, err := h.service.CreateTask(r.Context(), reqBody.Title)
	if err != nil {
		log.Error("handler: error creating task", "title", reqBody.Title, "error", err)
		pkg.WriteError(w, http.StatusInternalServerError, "error creating task")
		return
	}
	log.Info("handler: task created successfully", "id", createdTask.ID)
	pkg.WriteJSON(w, http.StatusCreated, createdTask)
}

func parseIntQueryParam(r *http.Request, key string, defaultValue int) (int, error) {
	valStr := r.URL.Query().Get(key)
	if valStr == "" {
		return defaultValue, nil
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return 0, err
	}
	return val, nil
}
