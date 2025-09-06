// Package robfigcronschedule provides a flexible cron scheduler implementation
// that works with the robfig/cron library. It supports complex scheduling
// scenarios including start dates, daily time windows, and precise interval control.
package robfigcronschedule

import (
	"log"
	"time"
)

// Schedule implements the robfig/cron.Schedule interface with enhanced features.
// It supports:
//   - Start dates (when the schedule becomes active)
//   - Daily time windows (startTime/endTime constraints)
//   - Flexible intervals with various time units
//   - Precision vs non-precision modes
//   - Before/after execution hooks
//
// Example usage:
//
//	// Run every 30 minutes during business hours, starting next Monday
//	schedule := New(
//		30, Minute,
//	    SetStartDate(nextMonday),
//	    SetStartTime(time.Date(0, 0, 0, 9, 0, 0, 0, time.UTC)),
//	    SetEndTime(time.Date(0, 0, 0, 17, 0, 0, 0, time.UTC)),
//	    EnablePrecision(),
//	)
type Schedule struct {
	// startDate controls when the schedule becomes active (optional)
	startDate *time.Time

	// startTime/endTime define daily time window constraints (optional)
	// If only startTime is set, endTime defaults to 23:59:59
	startTime *time.Time
	endTime   *time.Time

	// allowedWeekdays restricts execution to specific days of the week (optional)
	// If nil, the schedule can run on any day of the week.
	// If set, the schedule will only execute on the specified weekdays.
	// When combined with multi-day intervals (Week, Month, Year), this may cause
	// the schedule to skip to the next allowed day, potentially disrupting
	// the intended interval timing and triggering validation errors.
	allowedWeekdays *map[time.Weekday]bool

	// enabled controls whether the schedule is active
	enabled bool

	// interval and intervalTimeUnit control scheduling frequency
	// interval must be >= 1
	interval         int
	intervalTimeUnit IntervalTimeUnit

	// nextRun caches the next calculated run time for efficiency
	nextRun time.Time

	// precision controls scheduling behavior:
	// - true: strict interval adherence within time windows
	// - false: round up from startTime using intervals
	precision bool

	// Hook functions called before/after Next() calculations
	beforeNext func(*Schedule)
	afterNext  func(next *time.Time)
}

// Set updates the schedule with new options, validating the result.
// If validation fails, the schedule is rolled back to its previous state.
//
// Example:
//
//	err := schedule.Set(SetInterval(60), SetIntervalTimeUnit(Minute))
//	if err != nil {
//	    // Schedule unchanged, handle error
//	}
func (s *Schedule) Set(opts ...scheduleOption) error {
	// validate using temp var
	temp := &Schedule{
		enabled:          s.enabled,
		interval:         s.interval,
		intervalTimeUnit: s.intervalTimeUnit,
		precision:        s.precision,
	}

	// Only copy pointers that exist
	if s.startDate != nil {
		copy := *s.startDate
		temp.startDate = &copy
	}
	if s.startTime != nil {
		copy := *s.startTime
		temp.startTime = &copy
	}
	if s.endTime != nil {
		copy := *s.endTime
		temp.endTime = &copy
	}

	// For map, consider if we really need to back it up
	if s.allowedWeekdays != nil {
		copy := make(map[time.Weekday]bool, len(*s.allowedWeekdays))
		for k, v := range *s.allowedWeekdays {
			copy[k] = v
		}
		temp.allowedWeekdays = &copy
	}

	for _, opt := range opts {
		opt(temp)
	}

	if err := validate(temp); err != nil {
		return err
	}

	for _, opt := range opts {
		opt(s)
	}

	return nil
}

// Next returns the next scheduled run time relative to the given time t.
// This method implements the robfig/cron.Schedule interface.
//
// The evaluation follows this priority order:
//  1. Execute before-hook if set
//  2. If disabled, return t + 5 minutes (for periodic re-checking)
//  3. If nextRun is cached and still future, return it
//  4. If startDate is set and t is before it:
//     - Return startDate + startTime if both set
//     - Otherwise return startDate
//  5. If startTime is set (daily time window):
//     - Precision mode: strict intervals within window, next day if overflow
//     - Non-precision mode: round up from startTime using intervals
//  6. Otherwise: calculate next run using intervals from current time
//  7. Execute after-hook and cache result
//
// Time zones are handled by converting all times to t's location.
func (s *Schedule) Next(t time.Time) time.Time {
	//  1. Run pre-hook
	s.safeBeforeNext(s.beforeNext)

	//  2. If the schedule is disabled, schedule the next check 5 minutes later.
	if !s.enabled {
		return t.Add(5 * time.Minute)
	}

	//  3. If nextRun is still in the future, return it directly.
	if s.nextRun.After(t) {
		return s.nextRun
	}

	var next time.Time
	//  7. Run post-hook.
	defer s.safeAfterNext(s.afterNext, &next)

	//  4. If StartDate is set and t is before it:
	//     - If StartTime is also set and still in the future, return StartDate+StartTime.
	//     - Otherwise, return StartDate.
	if s.startDate != nil && t.Before(*s.startDate) {
		next = s.startDate.In(t.Location())
		if s.startTime != nil {
			localization := s.startTime.In(t.Location())
			next = time.Date(
				next.Year(),
				next.Month(),
				next.Day(),
				localization.Hour(),
				localization.Minute(),
				localization.Second(),
				localization.Nanosecond(),
				t.Location(),
			)

			return next
		}
		return next
	}

	//  5. If StartTime is set (time-of-day window):
	//     - If t is before today's STime, return today's STime.
	//     - If t is after today's ETime (or default 23:59:59), return tomorrow's STime.
	if s.startTime != nil {
		lStartTime := s.startTime.In(t.Location())

		startTime := time.Date(
			t.Year(),
			t.Month(),
			t.Day(),
			lStartTime.Hour(),
			lStartTime.Minute(),
			lStartTime.Second(),
			lStartTime.Nanosecond(),
			t.Location(),
		)

		var endTime time.Time
		if s.endTime != nil {
			lEndTime := s.endTime.In(t.Location())
			endTime = time.Date(
				t.Year(),
				t.Month(),
				t.Day(),
				lEndTime.Hour(),
				lEndTime.Minute(),
				lEndTime.Second(),
				lEndTime.Nanosecond(),
				t.Location(),
			)
		} else {
			endTime = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, t.Location())
		}

		// Check if today is an allowed day
		if !s.isDayAllowed(t) {
			// Skip to next allowed day at start time
			next = s.findNextAllowedDay(startTime.Add(24*time.Hour), true)
			return next
		}

		if s.precision {
			// use the earliest stime
			if t.Before(startTime) {
				next = startTime
				return next
			}

			// if current time is past the allowed endTime, use the earliest tomorrow startTime
			if t.After(endTime) {
				next = startTime.Add(24 * time.Hour)
				return next
			}

			// Within window - compute next interval but check bounds
			next = s.incrementInterval(t)
			if next.After(endTime) {
				// Past end time, move to next allowed day
				next = s.findNextAllowedDay(startTime.Add(24*time.Hour), true)
			}
			return next
		} else { // 6b. Otherwise, rounding next run based on the Interval and ItvUnit
			next = startTime
			for next.Before(t) {
				next = s.incrementInterval(next)
			}

			// If we've moved to a different day, check if it's allowed
			if next.Day() != t.Day() {
				next = s.findNextAllowedDay(next, true)
			}

			return next
		}
	}

	//  6a. Otherwise, compute the next run based on Interval and ItvUnit
	//     (seconds, minutes, hours, days, weeks, months, years).
	//     If no valid unit is provided, default to 5 minutes.
	next = s.incrementInterval(t)

	// Apply weekday filtering if the day changed
	if next.Day() != t.Day() || next.Month() != t.Month() || next.Year() != t.Year() {
		next = s.findNextAllowedDay(next, false)
	}

	return next
}

func (s *Schedule) incrementInterval(t time.Time) time.Time {
	switch s.intervalTimeUnit {
	case Second:
		return t.Add(time.Duration(s.interval) * time.Second)
	case Minute:
		return t.Add(time.Duration(s.interval) * time.Minute)
	case Hour:
		return t.Add(time.Duration(s.interval) * time.Hour)
	case Day:
		return t.AddDate(0, 0, s.interval)
	case Week:
		return t.AddDate(0, 0, s.interval*7)
	case Month:
		return t.AddDate(0, s.interval, 0)
	case Year:
		return t.AddDate(s.interval, 0, 0)
	default: // default 5 minutes
		return t.Add(5 * time.Minute)
	}
}

func (s *Schedule) setNextRun(nextRun *time.Time) {
	s.nextRun = *nextRun
}

func validate(s *Schedule) error {
	if s.interval < 1 {
		return ErrInvalidInterval
	}

	if s.startTime != nil && s.endTime != nil {
		startSeconds := s.startTime.Hour()*3600 + s.startTime.Minute()*60 + s.startTime.Second()
		endSeconds := s.endTime.Hour()*3600 + s.endTime.Minute()*60 + s.endTime.Second()
		if startSeconds >= endSeconds {
			return ErrInvalidTimeWindow
		}
	}

	if s.allowedWeekdays != nil && len(*s.allowedWeekdays) == 0 {
		return ErrNoDayInWeekdayWindow
	} else if s.allowedWeekdays != nil && len(*s.allowedWeekdays) > 0 { // If using week-based or longer intervals with weekday restrictions, warn about potential issues
		if s.intervalTimeUnit == Week || s.intervalTimeUnit == Month || s.intervalTimeUnit == Year {
			return ErrMultiIntervalWithWeekdayWindow
		}
	}

	return nil
}

// Check if today is an allowed day
func (s *Schedule) isDayAllowed(t time.Time) bool {
	if s.allowedWeekdays == nil {
		return true
	}

	return (*s.allowedWeekdays)[t.Weekday()]
}

// findNextAllowedDay finds the next day that matches the weekday criteria
// If preserveTime is true, it keeps the time-of-day; otherwise it may adjust it
func (s *Schedule) findNextAllowedDay(start time.Time, preserveTime bool) time.Time {
	// If no weekday restrictions, return as-is
	if s.allowedWeekdays == nil {
		return start
	}

	current := start

	// Safety limit to prevent infinite loops (check up to 14 days)
	for i := 0; i < 14; i++ {
		if s.isDayAllowed(current) {
			// If we want to preserve the original time and we have start/end times
			if preserveTime && s.startTime != nil {
				lStartTime := s.startTime.In(current.Location())
				return time.Date(
					current.Year(),
					current.Month(),
					current.Day(),
					lStartTime.Hour(),
					lStartTime.Minute(),
					lStartTime.Second(),
					lStartTime.Nanosecond(),
					current.Location(),
				)
			}
			return current
		}

		// Move to next day
		if preserveTime && s.startTime != nil {
			lStartTime := s.startTime.In(current.Location())
			// Jump to start time of next day
			current = time.Date(
				current.Year(),
				current.Month(),
				current.Day()+1,
				lStartTime.Hour(),
				lStartTime.Minute(),
				lStartTime.Second(),
				lStartTime.Nanosecond(),
				current.Location(),
			)
		} else {
			current = current.Add(24 * time.Hour)
		}
	}

	// Fallback: if no allowed day found in 2 weeks, return original time
	// This should never happen with valid configurations
	return start
}

// Handles beforeNext() panics
func (s *Schedule) safeBeforeNext(beforeNext func(*Schedule)) {
	if beforeNext == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("beforeNext() panicked. %v", r)
		}
	}()

	beforeNext(s)
}

// Handles afterNext() panics
func (s *Schedule) safeAfterNext(afterNext func(*time.Time), nextRun *time.Time) {
	defer s.setNextRun(nextRun)
	if afterNext == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("afterNext() panicked. %v", r)
		}
	}()
	afterNext(nextRun)
}
