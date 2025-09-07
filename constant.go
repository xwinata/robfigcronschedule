package robfigcronschedule

import "errors"

// IntervalTimeUnit represents the time unit for scheduling intervals.
type IntervalTimeUnit int

const (
	Second IntervalTimeUnit = iota
	Minute
	Hour
	Day
	Week
	Month
	Year
)

var (
	ErrInvalidInterval = errors.New(
		"invalid interval. interval cannot be less than 1",
	)
	ErrInvalidTimeWindow = errors.New(
		"invalid time window. start time must be before end time",
	)
	ErrMultiIntervalWithWeekdayWindow = errors.New(
		"multi weeks/months/years intervals with weekday restrictions may produce unexpected results",
	)
)
