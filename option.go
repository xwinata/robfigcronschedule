package robfigcronschedule

import "time"

type ScheduleOption func(*Schedule)

// SetStartTime sets the daily start time for the schedule.
// Combined with SetEndTime, this creates a daily time window.
// If endTime is not set, it defaults to 23:59:59.
// Pass nil to reset/remove the start time constraint.
//
// Examples:
//
//	// Set start time to 9 AM:
//	t := time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)
//	SetStartTime(&t)
//
//	// Reset start time (no constraint):
//	SetStartTime(nil)
func SetStartTime(t *time.Time) ScheduleOption {
	return func(s *Schedule) {
		s.startTime = t
	}
}

// SetEndTime sets the daily end time for the schedule.
// Must be used with SetStartTime to be meaningful.
// The schedule will not run after this time each day.
// Pass nil to reset/remove the end time constraint.
//
// Examples:
//
//	// Set end time to 5 PM:
//	t := time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC)
//	SetEndTime(&t)
//
//	// Reset end time (defaults to 23:59:59):
//	SetEndTime(nil)
func SetEndTime(t *time.Time) ScheduleOption {
	return func(s *Schedule) {
		s.endTime = t
	}
}

// SetStartDate sets when the schedule should begin executing.
// The schedule will not run before this date.
// Pass nil to reset/remove the start date constraint.
//
// Examples:
//
//	// Set start date to next Monday:
//	startDate := getNextMonday()
//	SetStartDate(&startDate)
//
//	// Set specific launch date:
//	launchTime := time.Date(2024, 12, 25, 0, 0, 0, 0, time.UTC)
//	SetStartDate(&launchTime)
//
//	// Reset start date (schedule active immediately):
//	SetStartDate(nil)
func SetStartDate(t *time.Time) ScheduleOption {
	return func(s *Schedule) {
		s.startDate = t
	}
}

// SetAllowedWeekdays restricts the schedule to run only on specified weekdays.
// If not set or if no weekdays are provided, the schedule can run on any day.
//
// When combined with multi-day intervals (Week, Month, Year), this may produce
// unexpected results as the schedule will skip to the next allowed day,
// potentially disrupting the intended interval timing.
//
// Examples:
//
//	// Run only on weekdays:
//	SetAllowedWeekdays(time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday)
//
//	// Run only on weekends:
//	SetAllowedWeekdays(time.Saturday, time.Sunday)
//
//	// Reset to allow any day:
//	SetAllowedWeekdays() // empty arguments
func SetAllowedWeekdays(weekdays ...time.Weekday) ScheduleOption {
	return func(s *Schedule) {
		if len(weekdays) < 1 {
			s.allowedWeekdays = nil
			return
		}

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
//
// Examples:
//
//	SetIntervalTimeUnit(Second)  // for second-based intervals
//	SetIntervalTimeUnit(Hour)    // for hour-based intervals
//	SetIntervalTimeUnit(Day)     // for day-based intervals
func SetIntervalTimeUnit(i IntervalTimeUnit) ScheduleOption {
	return func(s *Schedule) {
		s.intervalTimeUnit = i
	}
}

// SetBeforeNextFunc sets a function to call before each Next() calculation.
// Useful for logging, metrics, or state preparation.
// Pass nil to remove the hook.
//
// Examples:
//
//	// Set a logging hook:
//	SetBeforeNextFunc(func(s *Schedule) {
//	    log.Printf("Computing next run for interval: %d", s.interval)
//	})
//
//	// Remove the hook:
//	SetBeforeNextFunc(nil)
func SetBeforeNextFunc(f func(*Schedule)) ScheduleOption {
	return func(s *Schedule) {
		s.beforeNext = f
	}
}

// SetAfterNextFunc sets a function to call after each Next() calculation.
// The function receives a pointer to the calculated next run time.
// Useful for logging, metrics, or result processing.
// Pass nil to remove the hook.
//
// Examples:
//
//	// Set a logging hook:
//	SetAfterNextFunc(func(next *time.Time) {
//	    log.Printf("Next run scheduled for: %v", next)
//	})
//
//	// Remove the hook:
//	SetAfterNextFunc(nil)
func SetAfterNextFunc(f func(next *time.Time)) ScheduleOption {
	return func(s *Schedule) {
		s.afterNext = f
	}
}

// Enable activates the schedule (default state).
//
// Example:
//
//	Enable() // Activates the schedule
func Enable() ScheduleOption {
	return func(s *Schedule) {
		s.enabled = true
	}
}

// Disable deactivates the schedule.
// When disabled, Next() returns current time + 5 minutes, causing the cron job
// to run every 5 minutes but without executing the actual scheduled logic.
// This allows for periodic re-checking of the schedule state.
//
// Example:
//
//	Disable() // Schedule becomes inactive but cron job continues checking
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
//
// Example:
//
//	EnablePrecision() // Enables strict interval timing
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
//
// Example:
//
//	DisablePrecision() // Enables window-aligned timing
func DisablePrecision() ScheduleOption {
	return func(s *Schedule) {
		s.precision = false
	}
}
