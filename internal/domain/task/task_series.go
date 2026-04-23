package task

import (
	"fmt"
	"sort"
	"time"
)

type RecurrenceType string

const (
	RecurrenceDaily         RecurrenceType = "daily"
	RecurrenceMonthly       RecurrenceType = "monthly"
	RecurrenceSpecificDates RecurrenceType = "specific_dates"
	RecurrenceDayParity     RecurrenceType = "day_parity"
)

type DayParity string

const (
	DayParityOdd  DayParity = "odd"
	DayParityEven DayParity = "even"
)

type RecurrenceParams struct {
	EveryNDays int
	DayOfMonth int
	Dates      []time.Time
	DayParity  DayParity
}

type TaskSeries struct {
	ID               int64
	Title            string
	Description      string
	Status           Status
	RecurrenceType   RecurrenceType
	Params           *RecurrenceParams
	IsActive         bool
	StartedAt        time.Time
	GeneratedThrough time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (r RecurrenceType) Valid() bool {
	switch r {
	case RecurrenceDaily, RecurrenceMonthly, RecurrenceSpecificDates, RecurrenceDayParity:
		return true
	default:
		return false
	}
}

func (p DayParity) Valid() bool {
	switch p {
	case DayParityOdd, DayParityEven:
		return true
	default:
		return false
	}
}

func NormalizeRecurrence(recurrenceType RecurrenceType, params *RecurrenceParams, startedAt time.Time) (*RecurrenceParams, time.Time, error) {
	if !recurrenceType.Valid() {
		return nil, time.Time{}, fmt.Errorf("invalid recurrence type")
	}

	if params == nil {
		params = &RecurrenceParams{}
	}

	normalized := &RecurrenceParams{
		EveryNDays: params.EveryNDays,
		DayOfMonth: params.DayOfMonth,
		DayParity:  params.DayParity,
	}

	switch recurrenceType {
	case RecurrenceDaily:
		if normalized.EveryNDays <= 0 {
			return nil, time.Time{}, fmt.Errorf("every_n_days must be greater than zero")
		}
		if startedAt.IsZero() {
			return nil, time.Time{}, fmt.Errorf("starts_on is required for daily recurrence")
		}

	case RecurrenceMonthly:
		if normalized.DayOfMonth < 1 || normalized.DayOfMonth > 30 {
			return nil, time.Time{}, fmt.Errorf("day_of_month must be between 1 and 30")
		}
		if startedAt.IsZero() {
			return nil, time.Time{}, fmt.Errorf("starts_on is required for monthly recurrence")
		}

	case RecurrenceSpecificDates:
		if len(params.Dates) == 0 {
			return nil, time.Time{}, fmt.Errorf("dates must contain at least one value")
		}

		normalized.Dates = normalizeAndSortDates(params.Dates)
		if startedAt.IsZero() {
			startedAt = normalized.Dates[0]
		}

		hasFutureDate := false
		normalizedStart := NormalizeDate(startedAt)
		for _, date := range normalized.Dates {
			if !date.Before(normalizedStart) {
				hasFutureDate = true
				break
			}
		}
		if !hasFutureDate {
			return nil, time.Time{}, fmt.Errorf("all specified dates are before starts_on")
		}

	case RecurrenceDayParity:
		if !normalized.DayParity.Valid() {
			return nil, time.Time{}, fmt.Errorf("day_parity must be either odd or even")
		}
		if startedAt.IsZero() {
			return nil, time.Time{}, fmt.Errorf("starts_on is required for day parity recurrence")
		}
	}

	return normalized, NormalizeDate(startedAt), nil
}

func (s TaskSeries) OccurrenceDatesBetween(from, to time.Time) []time.Time {
	start := NormalizeDate(from)
	if seriesStart := NormalizeDate(s.StartedAt); start.Before(seriesStart) {
		start = seriesStart
	}

	end := NormalizeDate(to)
	if end.Before(start) {
		return nil
	}

	if s.RecurrenceType == RecurrenceSpecificDates {
		dates := make([]time.Time, 0, len(s.Params.Dates))
		for _, date := range s.Params.Dates {
			if date.Before(start) || date.After(end) {
				continue
			}
			dates = append(dates, date)
		}
		return dates
	}

	dates := make([]time.Time, 0)
	for current := start; !current.After(end); current = current.AddDate(0, 0, 1) {
		if s.OccursOn(current) {
			dates = append(dates, current)
		}
	}

	return dates
}

func (s TaskSeries) FirstOccurrenceOnOrAfter(from time.Time) (time.Time, bool) {
	start := NormalizeDate(from)
	if seriesStart := NormalizeDate(s.StartedAt); start.Before(seriesStart) {
		start = seriesStart
	}

	if s.RecurrenceType == RecurrenceSpecificDates {
		for _, date := range s.Params.Dates {
			if !date.Before(start) {
				return date, true
			}
		}
		return time.Time{}, false
	}

	limit := start.AddDate(10, 0, 0)
	for current := start; !current.After(limit); current = current.AddDate(0, 0, 1) {
		if s.OccursOn(current) {
			return current, true
		}
	}

	return time.Time{}, false
}

func (s TaskSeries) OccursOn(date time.Time) bool {
	if s.Params == nil {
		return false
	}

	current := NormalizeDate(date)
	start := NormalizeDate(s.StartedAt)
	if current.Before(start) {
		return false
	}

	switch s.RecurrenceType {
	case RecurrenceDaily:
		diffDays := int(current.Sub(start).Hours() / 24)
		return diffDays%s.Params.EveryNDays == 0
	case RecurrenceMonthly:
		return current.Day() == s.Params.DayOfMonth
	case RecurrenceSpecificDates:
		for _, date := range s.Params.Dates {
			if date.Equal(current) {
				return true
			}
		}
		return false
	case RecurrenceDayParity:
		if s.Params.DayParity == DayParityEven {
			return current.Day()%2 == 0
		}
		return current.Day()%2 != 0
	default:
		return false
	}
}

func NormalizeDate(value time.Time) time.Time {
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func normalizeAndSortDates(dates []time.Time) []time.Time {
	unique := make(map[time.Time]struct{}, len(dates))
	for _, date := range dates {
		unique[NormalizeDate(date)] = struct{}{}
	}

	result := make([]time.Time, 0, len(unique))
	for date := range unique {
		result = append(result, date)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Before(result[j])
	})

	return result
}
