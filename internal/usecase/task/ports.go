package task

import (
	"context"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Scope string

const (
	This             Scope = "this"
	ThisAndFollowing Scope = "this_and_following"
	All              Scope = "all"
)

type Repository interface {
	Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)
	CreateSeries(ctx context.Context, task *taskdomain.TaskSeries) (*taskdomain.TaskSeries, error)
	GetByID(ctx context.Context, id int64) (*taskdomain.Task, error)
	GetSeriesByID(ctx context.Context, id int64) (*taskdomain.TaskSeries, error)
	Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)
	UpdateSeries(ctx context.Context, series *taskdomain.TaskSeries) (*taskdomain.TaskSeries, error)
	Delete(ctx context.Context, id int64) error
	DeleteBySeries(ctx context.Context, seriesID int64) error
	UpdateTasksBySeries(ctx context.Context, seriesID int64, title, description string, status taskdomain.Status, updatedAt time.Time) error
	DeactivateSeries(ctx context.Context, seriesID int64, updatedAt time.Time) error
	List(ctx context.Context) ([]taskdomain.Task, error)
	ListActiveSeries(ctx context.Context) ([]taskdomain.TaskSeries, error)
}

type Usecase interface {
	Create(ctx context.Context, input CreateInput) (*taskdomain.Task, error)
	GetByID(ctx context.Context, id int64) (*taskdomain.Task, error)
	Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error)
	Delete(ctx context.Context, id int64, input DeleteInput) error
	List(ctx context.Context) ([]taskdomain.Task, error)
}

type CreateInput struct {
	Title            string
	Description      string
	Status           taskdomain.Status
	RecurrenceType   taskdomain.RecurrenceType
	RecurrenceParams *taskdomain.RecurrenceParams
	StartedAt        time.Time
}

type UpdateInput struct {
	Title       string
	Description string
	Status      taskdomain.Status
	Scope       Scope
}

type DeleteInput struct {
	Scope Scope
}

func (s Scope) Valid() bool {
	switch s {
	case This, ThisAndFollowing, All:
		return true
	default:
		return false
	}

}
