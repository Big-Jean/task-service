package handlers

import (
	"fmt"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

const dateLayout = "2006-01-02"

type createTaskRequestDTO struct {
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Status      taskdomain.Status  `json:"status"`
	Recurrence  *recurrenceDTO     `json:"recurrence,omitempty"`
}

type updateTaskRequestDTO struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Status      taskdomain.Status `json:"status"`
}

type recurrenceDTO struct {
	Type       taskdomain.RecurrenceType `json:"type"`
	EveryNDays *int                      `json:"every_n_days,omitempty"`
	DayOfMonth *int                      `json:"day_of_month,omitempty"`
	Dates      []string                  `json:"dates,omitempty"`
	DayParity  taskdomain.DayParity      `json:"day_parity,omitempty"`
	StartsOn   string                    `json:"starts_on,omitempty"`
}

type taskDTO struct {
	ID           int64             `json:"id"`
	TaskSeriesID *int64            `json:"task_series_id,omitempty"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	Status       taskdomain.Status `json:"status"`
	TaskDate     *time.Time        `json:"task_date,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

func newTaskDTO(task *taskdomain.Task) taskDTO {
	dto := taskDTO{
		ID:           task.ID,
		TaskSeriesID: task.TaskSeriesID,
		Title:        task.Title,
		Description:  task.Description,
		Status:       task.Status,
		CreatedAt:    task.CreatedAt,
		UpdatedAt:    task.UpdatedAt,
	}

	if !task.TaskDate.IsZero() {
		taskDate := task.TaskDate
		dto.TaskDate = &taskDate
	}

	return dto
}

func (r *recurrenceDTO) toDomain() (taskdomain.RecurrenceType, *taskdomain.RecurrenceParams, time.Time, error) {
	if r == nil {
		return "", nil, time.Time{}, nil
	}

	params := &taskdomain.RecurrenceParams{
		DayParity: r.DayParity,
	}
	if r.EveryNDays != nil {
		params.EveryNDays = *r.EveryNDays
	}
	if r.DayOfMonth != nil {
		params.DayOfMonth = *r.DayOfMonth
	}

	if len(r.Dates) > 0 {
		params.Dates = make([]time.Time, 0, len(r.Dates))
		for _, rawDate := range r.Dates {
			parsedDate, err := parseDate(rawDate)
			if err != nil {
				return "", nil, time.Time{}, fmt.Errorf("invalid recurrence date %q: %w", rawDate, err)
			}

			params.Dates = append(params.Dates, parsedDate)
		}
	}

	var startsOn time.Time
	var err error
	if r.StartsOn != "" {
		startsOn, err = parseDate(r.StartsOn)
		if err != nil {
			return "", nil, time.Time{}, fmt.Errorf("invalid starts_on: %w", err)
		}
	}

	return r.Type, params, startsOn, nil
}

func parseDate(value string) (time.Time, error) {
	parsed, err := time.Parse(dateLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("expected format %s", dateLayout)
	}

	return taskdomain.NormalizeDate(parsed), nil
}
