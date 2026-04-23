package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		INSERT INTO tasks (title, description, status, created_at, updated_at, task_series_id, task_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (task_series_id, task_date)
			WHERE task_series_id IS NOT NULL AND task_date IS NOT NULL
		DO UPDATE
		SET updated_at = tasks.updated_at
		RETURNING id, title, description, status, created_at, updated_at, task_series_id, task_date
	`

	row := r.pool.QueryRow(
		ctx,
		query,
		task.Title,
		task.Description,
		task.Status,
		task.CreatedAt,
		task.UpdatedAt,
		task.TaskSeriesID,
		nullTime(task.TaskDate),
	)

	created, err := scanTask(row)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (r *Repository) CreateSeries(ctx context.Context, series *taskdomain.TaskSeries) (*taskdomain.TaskSeries, error) {
	const query = `
		INSERT INTO task_series (
			title,
			description,
			status,
			recurrence_type,
			recurrence_params,
			is_active,
			started_at,
			generated_through,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, title, description, status, recurrence_type, recurrence_params, is_active, started_at, generated_through, created_at, updated_at
	`

	paramsJSON, err := marshalRecurrenceParams(series.Params)
	if err != nil {
		return nil, err
	}

	row := r.pool.QueryRow(
		ctx,
		query,
		series.Title,
		series.Description,
		series.Status,
		series.RecurrenceType,
		paramsJSON,
		series.IsActive,
		series.StartedAt,
		nullTime(series.GeneratedThrough),
		series.CreatedAt,
		series.UpdatedAt,
	)

	return scanTaskSeries(row)
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	const query = `
		SELECT id, title, description, status, created_at, updated_at, task_series_id, task_date
		FROM tasks
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	found, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return found, nil
}

func (r *Repository) GetSeriesByID(ctx context.Context, id int64) (*taskdomain.TaskSeries, error) {
	const query = `
		SELECT id, title, description, status, recurrence_type, recurrence_params, is_active, started_at, generated_through, created_at, updated_at
		FROM task_series
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	found, err := scanTaskSeries(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return found, nil
}

func (r *Repository) Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		UPDATE tasks
		SET title = $1,
			description = $2,
			status = $3,
			updated_at = $4,
			task_series_id = $5,
			task_date = $6
		WHERE id = $7
		RETURNING id, title, description, status, created_at, updated_at, task_series_id, task_date
	`

	row := r.pool.QueryRow(
		ctx,
		query,
		task.Title,
		task.Description,
		task.Status,
		task.UpdatedAt,
		task.TaskSeriesID,
		nullTime(task.TaskDate),
		task.ID,
	)

	updated, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return updated, nil
}

func (r *Repository) UpdateSeries(ctx context.Context, series *taskdomain.TaskSeries) (*taskdomain.TaskSeries, error) {
	const query = `
		UPDATE task_series
		SET title = $1,
			description = $2,
			status = $3,
			recurrence_type = $4,
			recurrence_params = $5,
			is_active = $6,
			started_at = $7,
			generated_through = $8,
			updated_at = $9
		WHERE id = $10
		RETURNING id, title, description, status, recurrence_type, recurrence_params, is_active, started_at, generated_through, created_at, updated_at
	`

	paramsJSON, err := marshalRecurrenceParams(series.Params)
	if err != nil {
		return nil, err
	}

	row := r.pool.QueryRow(
		ctx,
		query,
		series.Title,
		series.Description,
		series.Status,
		series.RecurrenceType,
		paramsJSON,
		series.IsActive,
		series.StartedAt,
		nullTime(series.GeneratedThrough),
		series.UpdatedAt,
		series.ID,
	)

	updated, err := scanTaskSeries(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return updated, nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM tasks WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return taskdomain.ErrNotFound
	}

	return nil
}

func (r *Repository) DeleteBySeries(ctx context.Context, seriesID int64) error {
	const query = `DELETE FROM tasks WHERE task_series_id = $1`

	if _, err := r.pool.Exec(ctx, query, seriesID); err != nil {
		return err
	}

	return nil
}

func (r *Repository) UpdateTasksBySeries(ctx context.Context, seriesID int64, title, description string, status taskdomain.Status, updatedAt time.Time) error {
	const query = `
		UPDATE tasks
		SET title = $1,
			description = $2,
			status = $3,
			updated_at = $4
		WHERE task_series_id = $5
	`

	if _, err := r.pool.Exec(ctx, query, title, description, status, updatedAt, seriesID); err != nil {
		return err
	}

	return nil
}

func (r *Repository) DeactivateSeries(ctx context.Context, seriesID int64, updatedAt time.Time) error {
	const query = `
		UPDATE task_series
		SET is_active = FALSE,
			updated_at = $1
		WHERE id = $2
	`

	result, err := r.pool.Exec(ctx, query, updatedAt, seriesID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return taskdomain.ErrNotFound
	}

	return nil
}

func (r *Repository) List(ctx context.Context) ([]taskdomain.Task, error) {
	const query = `
		SELECT id, title, description, status, created_at, updated_at, task_series_id, task_date
		FROM tasks
		ORDER BY task_date DESC NULLS LAST, id DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]taskdomain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, *task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (r *Repository) ListActiveSeries(ctx context.Context) ([]taskdomain.TaskSeries, error) {
	const query = `
		SELECT id, title, description, status, recurrence_type, recurrence_params, is_active, started_at, generated_through, created_at, updated_at
		FROM task_series
		WHERE is_active = TRUE
		ORDER BY id DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	series := make([]taskdomain.TaskSeries, 0)
	for rows.Next() {
		item, err := scanTaskSeries(rows)
		if err != nil {
			return nil, err
		}

		series = append(series, *item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return series, nil
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(scanner taskScanner) (*taskdomain.Task, error) {
	var (
		task         taskdomain.Task
		status       string
		taskSeriesID *int64
		taskDate     *time.Time
	)

	if err := scanner.Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&status,
		&task.CreatedAt,
		&task.UpdatedAt,
		&taskSeriesID,
		&taskDate,
	); err != nil {
		return nil, err
	}

	task.Status = taskdomain.Status(status)
	task.TaskSeriesID = taskSeriesID
	if taskDate != nil {
		task.TaskDate = *taskDate
	}

	return &task, nil
}

func scanTaskSeries(scanner taskScanner) (*taskdomain.TaskSeries, error) {
	var (
		series           taskdomain.TaskSeries
		status           string
		recurrenceType   string
		recurrenceBytes  []byte
		generatedThrough *time.Time
	)

	if err := scanner.Scan(
		&series.ID,
		&series.Title,
		&series.Description,
		&status,
		&recurrenceType,
		&recurrenceBytes,
		&series.IsActive,
		&series.StartedAt,
		&generatedThrough,
		&series.CreatedAt,
		&series.UpdatedAt,
	); err != nil {
		return nil, err
	}

	series.Status = taskdomain.Status(status)
	series.RecurrenceType = taskdomain.RecurrenceType(recurrenceType)
	if generatedThrough != nil {
		series.GeneratedThrough = *generatedThrough
	}

	params, err := unmarshalRecurrenceParams(recurrenceBytes)
	if err != nil {
		return nil, err
	}
	series.Params = params

	return &series, nil
}

func marshalRecurrenceParams(params *taskdomain.RecurrenceParams) ([]byte, error) {
	if params == nil {
		return json.Marshal(taskdomain.RecurrenceParams{})
	}

	return json.Marshal(params)
}

func unmarshalRecurrenceParams(raw []byte) (*taskdomain.RecurrenceParams, error) {
	if len(raw) == 0 {
		return &taskdomain.RecurrenceParams{}, nil
	}

	var params taskdomain.RecurrenceParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}

	return &params, nil
}

func nullTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}

	normalized := value
	return &normalized
}
