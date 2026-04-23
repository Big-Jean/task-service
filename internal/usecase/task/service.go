package task

import (
	"context"
	"fmt"
	"strings"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*taskdomain.Task, error) {
	normalized, err := validateCreateInput(input)
	if err != nil {
		return nil, err
	}

	if normalized.RecurrenceType == "" {
		now := s.now()
		model := &taskdomain.Task{
			Title:       normalized.Title,
			Description: normalized.Description,
			Status:      normalized.Status,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		return s.repo.Create(ctx, model)
	}

	now := s.now()
	series := &taskdomain.TaskSeries{
		Title:          normalized.Title,
		Description:    normalized.Description,
		Status:         normalized.Status,
		RecurrenceType: normalized.RecurrenceType,
		Params:         normalized.RecurrenceParams,
		IsActive:       true,
		StartedAt:      normalized.StartedAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	createdSeries, err := s.repo.CreateSeries(ctx, series)
	if err != nil {
		return nil, err
	}

	lastCreated, err := s.syncSeries(ctx, createdSeries, taskdomain.NormalizeDate(now))
	if err != nil {
		return nil, err
	}
	if lastCreated != nil {
		return lastCreated, nil
	}

	firstOccurrence, ok := createdSeries.FirstOccurrenceOnOrAfter(createdSeries.StartedAt)
	if !ok {
		return nil, fmt.Errorf("%w: recurrence does not produce any dates", ErrInvalidInput)
	}

	task := newTaskFromSeries(createdSeries, firstOccurrence, now)
	createdTask, err := s.repo.Create(ctx, task)
	if err != nil {
		return nil, err
	}

	createdSeries.GeneratedThrough = firstOccurrence
	createdSeries.UpdatedAt = now
	if _, err := s.repo.UpdateSeries(ctx, createdSeries); err != nil {
		return nil, err
	}

	return createdTask, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	return s.repo.GetByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	if !input.Scope.Valid() {
		return nil, fmt.Errorf("%w: scope must be valid", ErrInvalidInput)
	}

	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	normalized, err := validateUpdateInput(input)
	if err != nil {
		return nil, err
	}

	switch normalized.Scope {
	case This:
		task = &taskdomain.Task{
			ID:           id,
			TaskSeriesID: task.TaskSeriesID,
			Title:        normalized.Title,
			Description:  normalized.Description,
			Status:       normalized.Status,
			TaskDate:     task.TaskDate,
			CreatedAt:    task.CreatedAt,
			UpdatedAt:    s.now(),
		}

		return s.repo.Update(ctx, task)

	case All:
		if task.TaskSeriesID == nil {
			task = &taskdomain.Task{
				ID:           id,
				TaskSeriesID: nil,
				Title:        normalized.Title,
				Description:  normalized.Description,
				Status:       normalized.Status,
				TaskDate:     task.TaskDate,
				CreatedAt:    task.CreatedAt,
				UpdatedAt:    s.now(),
			}

			return s.repo.Update(ctx, task)
		}

		series, err := s.repo.GetSeriesByID(ctx, *task.TaskSeriesID)
		if err != nil {
			return nil, err
		}

		now := s.now()
		series.Title = normalized.Title
		series.Description = normalized.Description
		series.Status = normalized.Status
		series.UpdatedAt = now

		if _, err := s.repo.UpdateSeries(ctx, series); err != nil {
			return nil, err
		}

		if err := s.repo.UpdateTasksBySeries(ctx, series.ID, normalized.Title, normalized.Description, normalized.Status, now); err != nil {
			return nil, err
		}

		if _, err := s.syncSeries(ctx, series, taskdomain.NormalizeDate(now)); err != nil {
			return nil, err
		}

		return s.repo.GetByID(ctx, id)

	case ThisAndFollowing:
		return nil, fmt.Errorf("%w: scope this_and_following is not implemented yet", ErrInvalidInput)
	}

	return nil, fmt.Errorf("%w: invalid input", ErrInvalidInput)
}

func (s *Service) Delete(ctx context.Context, id int64, input DeleteInput) error {
	if id <= 0 {
		return fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	if !input.Scope.Valid() {
		return fmt.Errorf("%w: scope must be set", ErrInvalidInput)
	}

	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	switch input.Scope {
	case This:
		return s.repo.Delete(ctx, id)
	case All:
		if task.TaskSeriesID == nil {
			return s.repo.Delete(ctx, id)
		}

		if err := s.repo.DeleteBySeries(ctx, *task.TaskSeriesID); err != nil {
			return err
		}

		return s.repo.DeactivateSeries(ctx, *task.TaskSeriesID, s.now())
	case ThisAndFollowing:
		return fmt.Errorf("%w: scope this_and_following is not implemented yet", ErrInvalidInput)
	}

	return fmt.Errorf("%w: invalid input", ErrInvalidInput)
}

func (s *Service) List(ctx context.Context) ([]taskdomain.Task, error) {
	series, err := s.repo.ListActiveSeries(ctx)
	if err != nil {
		return nil, err
	}

	target := taskdomain.NormalizeDate(s.now())
	for i := range series {
		if _, err := s.syncSeries(ctx, &series[i], target); err != nil {
			return nil, err
		}
	}

	return s.repo.List(ctx)
}

func (s *Service) syncSeries(ctx context.Context, series *taskdomain.TaskSeries, target time.Time) (*taskdomain.Task, error) {
	if !series.IsActive {
		return nil, nil
	}

	from := taskdomain.NormalizeDate(series.StartedAt)
	if !series.GeneratedThrough.IsZero() {
		next := taskdomain.NormalizeDate(series.GeneratedThrough).AddDate(0, 0, 1)
		if next.After(from) {
			from = next
		}
	}

	target = taskdomain.NormalizeDate(target)
	if target.Before(from) {
		return nil, nil
	}

	now := s.now()
	occurrenceDates := series.OccurrenceDatesBetween(from, target)
	var lastCreated *taskdomain.Task
	for _, occurrenceDate := range occurrenceDates {
		task := newTaskFromSeries(series, occurrenceDate, now)

		created, err := s.repo.Create(ctx, task)
		if err != nil {
			return nil, err
		}

		lastCreated = created
	}

	series.GeneratedThrough = target
	series.UpdatedAt = now
	if _, err := s.repo.UpdateSeries(ctx, series); err != nil {
		return nil, err
	}

	return lastCreated, nil
}

func newTaskFromSeries(series *taskdomain.TaskSeries, taskDate, now time.Time) *taskdomain.Task {
	return &taskdomain.Task{
		TaskSeriesID: &series.ID,
		Title:        series.Title,
		Description:  series.Description,
		Status:       series.Status,
		TaskDate:     taskdomain.NormalizeDate(taskDate),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func validateCreateInput(input CreateInput) (CreateInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		return CreateInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	if input.Status == "" {
		input.Status = taskdomain.StatusNew
	}

	if !input.Status.Valid() {
		return CreateInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}

	if input.RecurrenceType == "" {
		if input.RecurrenceParams != nil || !input.StartedAt.IsZero() {
			return CreateInput{}, fmt.Errorf("%w: recurrence_type is required when recurrence settings are provided", ErrInvalidInput)
		}
		return input, nil
	}

	params, startedAt, err := taskdomain.NormalizeRecurrence(input.RecurrenceType, input.RecurrenceParams, input.StartedAt)
	if err != nil {
		return CreateInput{}, fmt.Errorf("%w: %s", ErrInvalidInput, err)
	}

	input.RecurrenceParams = params
	input.StartedAt = startedAt

	return input, nil
}

func validateUpdateInput(input UpdateInput) (UpdateInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		return UpdateInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	if !input.Status.Valid() {
		return UpdateInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}

	return input, nil
}
