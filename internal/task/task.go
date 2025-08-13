package task

import "context"

type Task struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}
type Repository interface {
	Create(ctx context.Context, task *Task) (*Task, error)
	GetByID(ctx context.Context, id string) (*Task, error)
	GetAll(ctx context.Context, statusFilter string, limit, offset int) ([]*Task, error)
}

type Service interface {
	CreateTask(ctx context.Context, title string) (*Task, error)
	GetTaskByID(ctx context.Context, id string) (*Task, error)
	GetAllTasks(ctx context.Context, statusFilter string, limit, offset int) ([]*Task, error)
}
