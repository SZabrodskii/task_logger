package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"task_api/internal/logger"
	"task_api/internal/task"
)

var ErrNotFound = errors.New("task not found")

type TaskRepository struct {
	mu            sync.RWMutex
	tasksByID     map[int64]*task.Task
	tasksByStatus map[string][]int64
	allTasksIDs   []int64
	counter       int64
	log           logger.Logger
}

func NewTaskRepository(log logger.Logger) *TaskRepository {
	return &TaskRepository{
		tasksByID:     make(map[int64]*task.Task),
		tasksByStatus: make(map[string][]int64),
		allTasksIDs:   make([]int64, 0),
		counter:       0,
		log:           log,
	}
}
func (r *TaskRepository) Create(ctx context.Context, t *task.Task) (*task.Task, error) {
	log := logger.FromContext(ctx).With("where", "repository")
	log.Debug("repository: creating task", "title", t.Title)
	r.mu.Lock()
	defer r.mu.Unlock()

	r.counter++
	t.ID = r.counter

	r.tasksByID[t.ID] = t
	r.tasksByStatus[t.Status] = append(r.tasksByStatus[t.Status], t.ID)
	r.allTasksIDs = append(r.allTasksIDs, t.ID)

	log.Debug("repository: task created successfully", "id", t.ID)
	return t, nil
}

func (r *TaskRepository) GetByID(ctx context.Context, id string) (*task.Task, error) {
	log := logger.FromContext(ctx).With("where", "repository")
	log.Debug("repository: getting task by id", "id", id)
	r.mu.RLock()
	defer r.mu.RUnlock()

	taskID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID format: %w", err)
	}

	t, ok := r.tasksByID[taskID]
	if !ok {
		return nil, ErrNotFound
	}

	log.Debug("repository: task retrieved from repository", "id", t.ID)
	return t, nil
}

func (r *TaskRepository) GetAll(ctx context.Context, statusFilter string, limit, offset int) ([]*task.Task, error) {
	log := logger.FromContext(ctx).With("where", "repository")
	log.Debug("repository: getting all tasks", "status_filter", statusFilter, "limit", limit, "offset", offset)

	r.mu.RLock()
	defer r.mu.RUnlock()

	var taskIDs []int64

	if statusFilter != "" {
		taskIDs = r.tasksByStatus[statusFilter]
	} else {
		taskIDs = r.allTasksIDs
	}
	if offset >= len(taskIDs) {
		return []*task.Task{}, nil
	}
	end := offset + limit
	if end > len(taskIDs) {
		end = len(taskIDs)
	}
	paginateIDs := taskIDs[offset:end]

	result := make([]*task.Task, 0, len(paginateIDs))
	for _, id := range paginateIDs {
		result = append(result, r.tasksByID[id])
	}

	log.Debug("repository: tasks retrieved from repository", "count", len(result))
	return result, nil

}
