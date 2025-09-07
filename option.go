package robfigcronschedule

import "time"

type ScheduleOption func(*Schedule)

// SetStartTime sets the daily start time for the schedule.
// Combined with SetEndTime, this creates a daily time window.
// If endTime is not set, it defaults to 23:59:59.
//
// Example: SetStartTime(time.Date(0, 0, 0, 9, 0, 0, 0, time.UTC))
// creates a start time of 9:00 AM.
func SetStartTime(t time.Time) ScheduleOption {
	return func(s *Schedule) {
		s.startTime = &t
	}
}

// SetEndTime sets the daily end time for the schedule.
// Must be used with SetStartTime to be meaningful.
// The schedule will not run after this time each day.
func SetEndTime(t time.Time) ScheduleOption {
	return func(s *Schedule) {
		s.endTime = &t
	}
}

// SetStartDate sets when the schedule should begin executing.
// The schedule will not run before this date.
func SetStartDate(t time.Time) ScheduleOption {
	return func(s *Schedule) {
		s.startDate = &t
	}
}

// SetAllowedWeekdays restricts the schedule to run only on specified weekdays.
// If not set, the schedule can run on any day of the week.
//
// When combined with multi-day intervals (Week, Month, Year), this may produce
// unexpected results as the schedule will skip to the next allowed day,
// potentially disrupting the intended interval timing.
func SetAllowedWeekdays(weekdays ...time.Weekday) ScheduleOption {
	return func(s *Schedule) {
		allowed := make(map[time.Weekday]bool)
		for _, day := range weekdays {
			allowed[day] = true
		}
		s.allowedWeekdays = &allowed
	}
}

// SetInterval override how often the schedule should run.
// Must be >= 1. Use with SetIntervalTimeUnit to specify the unit.
//
// Examples:
//
//	SetInterval(30) + SetIntervalTimeUnit(Minute) = every 30 minutes
//	SetInterval(2) + SetIntervalTimeUnit(Hour) = every 2 hours
func SetInterval(i int) ScheduleOption {
	return func(s *Schedule) {
		s.interval = i
	}
}

// SetIntervalTimeUnit override the time unit for intervals.
// Use one of: Second, Minute, Hour, Day, Week, Month, Year
func SetIntervalTimeUnit(i IntervalTimeUnit) ScheduleOption {
	return func(s *Schedule) {
		s.intervalTimeUnit = i
	}
}

// SetBeforeNextFunc sets a function to call before each Next() calculation.
// Useful for logging, metrics, or state preparation.
func SetBeforeNextFunc(f func(*Schedule)) ScheduleOption {
	return func(s *Schedule) {
		s.beforeNext = f
	}
}

// SetAfterNextFunc sets a function to call after each Next() calculation.
// The function receives a pointer to the calculated next run time.
// Useful for logging, metrics, or result processing.
func SetAfterNextFunc(f func(next *time.Time)) ScheduleOption {
	return func(s *Schedule) {
		s.afterNext = f
	}
}

// Enable activates the schedule (default state).
func Enable() ScheduleOption {
	return func(s *Schedule) {
		s.enabled = true
	}
}

// Disable deactivates the schedule.
// When disabled, Next() returns current time + 5 minutes for periodic re-checking.
func Disable() ScheduleOption {
	return func(s *Schedule) {
		s.enabled = false
	}
}

// EnablePrecision enables precision mode scheduling (default).
// In precision mode:
// - Intervals are calculated strictly from the current time
// - If the next interval exceeds the daily time window, it moves to the next day
// - Provides more predictable timing but may skip time slots
func EnablePrecision() ScheduleOption {
	return func(s *Schedule) {
		s.precision = true
	}
}

// DisablePrecision disables precision mode.
// In non-precision mode:
// - Intervals are calculated by rounding up from startTime
// - Ensures no time slots are missed within the daily window
// - May have slight timing variations
func DisablePrecision() ScheduleOption {
	return func(s *Schedule) {
		s.precision = false
	}
}
