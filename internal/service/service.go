package service

import (
	"context"
	"fmt"
	"task_api/internal/logger"
	"task_api/internal/task"
)

type TaskService struct {
	repository task.Repository
	log        logger.Logger
}

func NewTaskService(repo task.Repository, log logger.Logger) *TaskService {
	return &TaskService{
		repository: repo,
		log:        log,
	}
}

func (s *TaskService) CreateTask(ctx context.Context, title string) (*task.Task, error) {
	log := logger.FromContext(ctx).With("where", "service")
	log.Debug("service: creating task", "title", title)

	t := &task.Task{
		Title:  title,
		Status: "new",
	}

	createdTask, err := s.repository.Create(ctx, t)
	if err != nil {
		return nil, fmt.Errorf("service: error creating task: %w", err)
	}

	log.Debug("service: task created successfully", "id", createdTask.ID, "title", title)
	return createdTask, nil
}

func (s *TaskService) GetTaskByID(ctx context.Context, id string) (*task.Task, error) {
	log := logger.FromContext(ctx).With("where", "service")
	log.Debug("service: getting task by id", "id", id)
	t, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("service: error getting task: %w", err)
	}
	log.Debug("service: task retrieved from repository", "id", t.ID)
	return t, nil
}

func (s *TaskService) GetAllTasks(ctx context.Context, statusFilter string, limit, offset int) ([]*task.Task, error) {
	log := logger.FromContext(ctx).With("where", "service")
	log.Debug("service: getting all tasks", "status_filter", statusFilter, "limit", limit, "offset", offset)
	tasks, err := s.repository.GetAll(ctx, statusFilter, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("service: error getting all tasks: %w", err)
	}
	log.Debug("service: tasks retrieved from repository", "count", len(tasks))
	return tasks, nil
}
