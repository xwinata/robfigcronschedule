package robfigcronschedule

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_BasicValidation(t *testing.T) {
	startTime := time.Date(2000, 1, 1, 10, 0, 0, 0, time.UTC)
	endTime := time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		interval      int
		unit          IntervalTimeUnit
		opts          []ScheduleOption
		expectError   error
		expectEnabled bool
	}{
		{
			name:          "valid basic schedule",
			interval:      5,
			unit:          Second,
			expectEnabled: true,
		},
		{
			name:        "invalid negative interval",
			interval:    -1,
			unit:        Second,
			expectError: ErrInvalidInterval,
		},
		{
			name:        "invalid zero interval",
			interval:    0,
			unit:        Second,
			expectError: ErrInvalidInterval,
		},
		{
			name:     "invalid time window",
			interval: 5,
			unit:     Second,
			opts: []ScheduleOption{
				SetStartTime(&startTime),
				SetEndTime(&endTime),
			},
			expectError: ErrInvalidTimeWindow,
		},
		{
			name:     "empty weekdays",
			interval: 5,
			unit:     Second,
			opts: []ScheduleOption{
				SetAllowedWeekdays(), // empty
			},
			expectEnabled: true,
			expectError:   nil,
		},
		{
			name:     "multi-week interval with weekday restriction",
			interval: 2,
			unit:     Week,
			opts: []ScheduleOption{
				SetAllowedWeekdays(time.Monday),
			},
			expectError: ErrMultiIntervalWithWeekdayWindow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule, err := New(tt.interval, tt.unit, tt.opts...)

			if tt.expectError != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.expectError)
				assert.Nil(t, schedule)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, schedule)
				assert.Equal(t, tt.expectEnabled, schedule.enabled)
			}
		})
	}
}

func TestSchedule_BusinessHoursProcessing(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		expected string
	}{
		{
			name:     "before business hours",
			current:  "2024-03-11 08:59:58", // Monday 8:59:58 AM
			expected: "2024-03-11 09:00:00", // Monday 9:00 AM
		},
		{
			name:     "during business hours",
			current:  "2024-03-11 10:30:00", // Monday 10:30 AM
			expected: "2024-03-11 10:30:02", // Monday 10:30:02 AM (2 sec later)
		},
		{
			name:     "end of business day",
			current:  "2024-03-11 16:59:59", // Monday 4:59:59 PM
			expected: "2024-03-12 09:00:00", // Tuesday 9:00 AM (next day)
		},
		{
			name:     "weekend - skip to monday",
			current:  "2024-03-10 10:00:00", // Sunday 10:00 AM
			expected: "2024-03-11 09:00:00", // Monday 9:00 AM
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startTime := time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)
			endTime := time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC)
			// Real-world use case: Process data every 2 seconds during business hours
			schedule, err := New(
				2,
				Second,
				SetStartTime(&startTime),
				SetEndTime(&endTime),
				SetAllowedWeekdays(
					time.Monday,
					time.Tuesday,
					time.Wednesday,
					time.Thursday,
					time.Friday,
				),
				EnablePrecision(),
			)
			require.NoError(t, err)

			current := parseTime(t, tt.current)
			expected := parseTime(t, tt.expected)

			next := schedule.Next(current)
			assert.Equal(t, expected, next)
		})
	}
}

func TestSchedule_MaintenanceWindow(t *testing.T) {
	startTime := time.Date(2000, 1, 1, 2, 0, 0, 0, time.UTC)
	// Real-world use case: Daily maintenance every day at 2 AM
	schedule, err := New(1, Day,
		SetStartTime(&startTime),
	)
	require.NoError(t, err)

	tests := []struct {
		name     string
		current  string
		expected string
	}{
		{
			name:     "before maintenance",
			current:  "2024-03-12 01:30:00", // Tuesday 1:30 AM
			expected: "2024-03-12 02:00:00", // Tuesday 2:00 AM
		},
		{
			name:     "after maintenance",
			current:  "2024-03-12 03:00:00", // Tuesday 3:00 AM
			expected: "2024-03-13 02:00:00", // Wednesday 2:00 AM
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := parseTime(t, tt.current)
			expected := parseTime(t, tt.expected)

			next := schedule.Next(current)
			assert.Equal(t, expected, next)
		})
	}
}

func TestSchedule_DatabaseConfigUpdate(t *testing.T) {
	// Mock database config
	mockDB := struct {
		IntervalSeconds int
		Enabled         bool
		Version         int
	}{
		IntervalSeconds: 3,
		Enabled:         true,
		Version:         1,
	}

	var hookCallCount int
	var lastInterval int
	var lastEnabled bool

	// Create schedule with mock database config updater
	schedule, err := New(5, Second, // Initial config different from "DB"
		SetBeforeNextFunc(func(s *Schedule) {
			hookCallCount++

			// Simulate database query result
			lastInterval = mockDB.IntervalSeconds
			lastEnabled = mockDB.Enabled

			// Apply updates if needed
			var updates []ScheduleOption
			if mockDB.IntervalSeconds != s.interval {
				updates = append(updates, SetInterval(mockDB.IntervalSeconds))
			}
			if mockDB.Enabled != s.enabled {
				if mockDB.Enabled {
					updates = append(updates, Enable())
				} else {
					updates = append(updates, Disable())
				}
			}

			if len(updates) > 0 {
				s.Set(updates...)
			}
		}),
	)
	require.NoError(t, err)

	// Test initial state
	now := time.Now()
	schedule.Next(now)

	// Hook should have been called
	assert.Equal(t, 1, hookCallCount)
	assert.Equal(t, 3, lastInterval)      // From "database"
	assert.True(t, lastEnabled)           // From "database"
	assert.Equal(t, 3, schedule.interval) // Should be updated to DB value

	// Simulate config change in "database"
	mockDB.IntervalSeconds = 7
	mockDB.Enabled = false
	mockDB.Version = 2

	// Call Next again to trigger hook
	schedule.Next(now.Add(time.Second))

	assert.Equal(t, 2, hookCallCount)
	assert.Equal(t, 7, lastInterval)
	assert.False(t, lastEnabled)
	assert.Equal(t, 7, schedule.interval)
	assert.False(t, schedule.enabled)
}

func TestSchedule_FeatureFlagIntegration(t *testing.T) {
	// Mock feature flag client
	mockFlags := map[string]interface{}{
		"schedule_interval": 2,
		"schedule_disabled": false,
	}

	var hookCallCount int

	schedule, err := New(5, Second,
		SetBeforeNextFunc(func(s *Schedule) {
			hookCallCount++

			// Check feature flags
			if disabled, ok := mockFlags["schedule_disabled"].(bool); ok && disabled {
				if s.enabled {
					s.Set(Disable())
				}
				return
			}

			// Check interval override
			if interval, ok := mockFlags["schedule_interval"].(int); ok && interval != s.interval {
				s.Set(SetInterval(interval))
			}

			// Re-enable if disabled
			if !s.enabled {
				s.Set(Enable())
			}
		}),
	)
	require.NoError(t, err)

	now := time.Now()

	// Initial run - should update interval to 2
	schedule.Next(now)
	assert.Equal(t, 1, hookCallCount)
	assert.Equal(t, 2, schedule.interval)
	assert.True(t, schedule.enabled)

	// Disable via feature flag
	mockFlags["schedule_disabled"] = true
	schedule.Next(now.Add(time.Second))
	assert.Equal(t, 2, hookCallCount)
	assert.False(t, schedule.enabled)

	// Re-enable via feature flag
	mockFlags["schedule_disabled"] = false
	schedule.Next(now.Add(2 * time.Second))
	assert.Equal(t, 3, hookCallCount)
	assert.True(t, schedule.enabled)

	// Change interval via feature flag
	mockFlags["schedule_interval"] = 8
	schedule.Next(now.Add(3 * time.Second))
	assert.Equal(t, 4, hookCallCount)
	assert.Equal(t, 8, schedule.interval)
	assert.True(t, schedule.enabled)
}

func TestSchedule_DisabledSchedule(t *testing.T) {
	schedule, err := New(5, Second, Disable())
	require.NoError(t, err)

	now := time.Now()
	next := schedule.Next(now)

	// Disabled schedule should return current time + 5 minutes
	expected := now.Add(5 * time.Minute)
	assert.WithinDuration(t, expected, next, time.Second)
}

func TestSchedule_StartDateFuture(t *testing.T) {
	dubaiLoc, _ := time.LoadLocation("Asia/Dubai")
	futureDate := time.Date(2025, 3, 15, 14, 30, 0, 0, time.Local)
	now := time.Date(2025, 3, 14, 10, 0, 0, 0, dubaiLoc)
	startTime := time.Date(2000, 1, 1, 2, 0, 0, 0, time.UTC)

	schedule, err := New(5, Second,
		SetStartDate(&futureDate),
		SetStartTime(&startTime),
	)
	require.NoError(t, err)

	next := schedule.Next(now)

	// Should return start date + start time
	expectedTime := time.Date(
		futureDate.Year(), futureDate.Month(), futureDate.Day(),
		6, 0, 0, 0, dubaiLoc,
	)
	assert.Equal(t, expectedTime, next)
}

func TestSchedule_PrecisionModes(t *testing.T) {
	startTime := time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)
	endTime := time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		precision bool
		current   string
		expected  string
	}{
		{
			name:      "precision mode - strict intervals",
			precision: true,
			current:   "2024-03-11 10:00:01",
			expected:  "2024-03-11 10:00:03", // 2 seconds later
		},
		{
			name:      "non-precision mode - aligned to start time",
			precision: false,
			current:   "2024-03-11 10:00:01",
			expected:  "2024-03-11 10:00:02", // Next 2-sec slot from 9:00
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := []ScheduleOption{
				SetStartTime(&startTime),
				SetEndTime(&endTime),
			}

			if tt.precision {
				opts = append(opts, EnablePrecision())
			} else {
				opts = append(opts, DisablePrecision())
			}

			schedule, err := New(2, Second, opts...)
			require.NoError(t, err)

			current := parseTime(t, tt.current)
			expected := parseTime(t, tt.expected)

			next := schedule.Next(current)
			assert.Equal(t, expected, next)
		})
	}
}

func TestSchedule_IntervalTypes(t *testing.T) {
	now := parseTime(t, "2024-03-11 10:00:00")

	tests := []struct {
		name     string
		interval int
		unit     IntervalTimeUnit
		expected string
	}{
		{
			name:     "5 seconds",
			interval: 5,
			unit:     Second,
			expected: "2024-03-11 10:00:05",
		},
		{
			name:     "2 minutes",
			interval: 2,
			unit:     Minute,
			expected: "2024-03-11 10:02:00",
		},
		{
			name:     "1 hour",
			interval: 1,
			unit:     Hour,
			expected: "2024-03-11 11:00:00",
		},
		{
			name:     "2 days",
			interval: 2,
			unit:     Day,
			expected: "2024-03-13 00:00:00",
		},
		{
			name:     "1 week",
			interval: 1,
			unit:     Week,
			expected: "2024-03-18 00:00:00",
		},
		{
			name:     "1 month",
			interval: 1,
			unit:     Month,
			expected: "2024-04-11 00:00:00",
		},
		{
			name:     "1 year",
			interval: 1,
			unit:     Year,
			expected: "2025-03-11 00:00:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule, err := New(tt.interval, tt.unit)
			require.NoError(t, err)

			expected := parseTime(t, tt.expected)
			next := schedule.Next(now)

			assert.Equal(t, expected, next)
		})
	}
}

func TestSchedule_HookPanicRecovery(t *testing.T) {
	var beforeCalled bool
	var afterCalled bool

	schedule, err := New(5, Second,
		SetBeforeNextFunc(func(s *Schedule) {
			beforeCalled = true
			panic("test panic in beforeNext")
		}),
		SetAfterNextFunc(func(next *time.Time) {
			afterCalled = true
			panic("test panic in afterNext")
		}),
	)
	require.NoError(t, err)

	now := time.Now()

	// Should not panic despite hooks panicking
	next := schedule.Next(now)

	assert.True(t, beforeCalled)
	assert.True(t, afterCalled)
	assert.True(t, next.After(now))
}

func TestSchedule_SetConfigValidation(t *testing.T) {
	schedule, err := New(5, Second)
	require.NoError(t, err)

	// Valid update
	err = schedule.Set(SetInterval(10))
	assert.NoError(t, err)
	assert.Equal(t, 10, schedule.interval)

	// Invalid update should not change schedule
	originalInterval := schedule.interval
	err = schedule.Set(SetInterval(-5))
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidInterval)
	assert.Equal(t, originalInterval, schedule.interval) // Unchanged
}

func TestSchedule_WeekdayFiltering(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		expected string
	}{
		{
			name:     "friday to monday",
			current:  "2024-03-15 10:00:00", // Friday
			expected: "2024-03-18 09:00:00", // Monday
		},
		{
			name:     "saturday to monday",
			current:  "2024-03-16 10:00:00", // Saturday
			expected: "2024-03-18 09:00:00", // Monday
		},
		{
			name:     "sunday to monday",
			current:  "2024-03-17 10:00:00", // Sunday
			expected: "2024-03-18 09:00:00", // Monday
		},
		{
			name:     "tuesday to wednesday",
			current:  "2024-03-12 10:00:00", // Tuesday
			expected: "2024-03-13 09:00:00", // Wednesday
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startTime := time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)
			schedule, err := New(
				1,
				Day,
				SetStartTime(&startTime),
				SetAllowedWeekdays(
					time.Monday,
					time.Tuesday,
					time.Wednesday,
					time.Thursday,
					time.Friday,
				),
			)
			require.NoError(t, err)

			current := parseTime(t, tt.current)
			expected := parseTime(t, tt.expected)

			next := schedule.Next(current)
			assert.Equal(t, expected, next)
		})
	}
}

func TestSchedule_NilTimeReset(t *testing.T) {
	startTime := time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)
	schedule, err := New(30, Minute, SetStartTime(&startTime))
	require.NoError(t, err)

	// Reset to nil
	err = schedule.Set(SetStartTime(nil))
	require.NoError(t, err)
}

// Helper functions
func parseTime(t *testing.T, timeStr string) time.Time {
	parsed, err := time.Parse("2006-01-02 15:04:05", timeStr)
	require.NoError(t, err)
	return parsed.UTC()
}

func TestSchedule_CriticalEdgeCases(t *testing.T) {
	t.Run("late night with restricted next day", func(t *testing.T) {
		// Test case 1: run at 23:50 with 15-minute interval, tomorrow is not an allowed day
		// The next run should be the day after tomorrow at 00:00

		current := parseTime(t, "2024-03-11 23:50:00") // Monday 23:50

		schedule, err := New(
			15,
			Minute,
			SetAllowedWeekdays(time.Monday, time.Wednesday, time.Friday), // Tuesday not allowed
		)
		require.NoError(t, err)

		next := schedule.Next(current)

		// Should skip Tuesday (not allowed) and go to Wednesday 00:00
		expected := parseTime(t, "2024-03-13 00:00:00") // Wednesday 00:00
		assert.Equal(t, expected, next,
			"Should skip disallowed Tuesday and go to Wednesday midnight")
	})

	t.Run("end of time window boundary", func(t *testing.T) {
		// Test case 2: run at 18:00 with 15min interval, endTime is 18:10, startTime is 8:00
		// The next run should be tomorrow at 8:00

		current := parseTime(t, "2024-03-11 18:00:00") // Monday 18:00

		startTime := time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC) // 8:00 AM
		endTime := time.Date(2000, 1, 1, 18, 10, 0, 0, time.UTC) // 6:10 PM

		schedule, err := New(
			15,
			Minute,
			SetStartTime(&startTime),
			SetEndTime(&endTime),
			EnablePrecision(), // Use precision mode for strict interval timing
		)
		require.NoError(t, err)

		next := schedule.Next(current)

		// Next 15-minute interval would be 18:15, but that's past endTime (18:10)
		// So should move to tomorrow at startTime (8:00)
		expected := parseTime(t, "2024-03-12 08:00:00") // Tuesday 8:00 AM
		assert.Equal(t, expected, next,
			"Should move to next day startTime when interval exceeds endTime")
	})

	t.Run("late night with time window and restricted next day", func(t *testing.T) {
		// Combined edge case: late night + time window + weekday restriction
		// Run at 23:50 with 15min interval, window 8:00-18:10, tomorrow not allowed
		// Should skip to day after tomorrow at 8:00

		current := parseTime(t, "2024-03-11 23:50:00") // Monday 23:50

		startTime := time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC)
		endTime := time.Date(2000, 1, 1, 18, 10, 0, 0, time.UTC)

		schedule, err := New(
			15,
			Minute,
			SetStartTime(&startTime),
			SetEndTime(&endTime),
			SetAllowedWeekdays(time.Monday, time.Wednesday, time.Friday), // Tuesday not allowed
			EnablePrecision(),
		)
		require.NoError(t, err)

		next := schedule.Next(current)

		// Should skip Tuesday (not allowed) and go to Wednesday at startTime
		expected := parseTime(t, "2024-03-13 08:00:00") // Wednesday 8:00 AM
		assert.Equal(t, expected, next,
			"Should skip disallowed day and use startTime of next allowed day")
	})

	t.Run("end of time window on friday", func(t *testing.T) {
		// Edge case: end of time window on Friday, weekends not allowed
		// Should jump to Monday startTime

		current := parseTime(t, "2024-03-15 18:05:00") // Friday 18:05

		startTime := time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC) // 9:00 AM
		endTime := time.Date(2000, 1, 1, 18, 10, 0, 0, time.UTC) // 6:10 PM

		schedule, err := New(
			10,
			Minute,
			SetStartTime(&startTime),
			SetEndTime(&endTime),
			SetAllowedWeekdays(
				time.Monday,
				time.Tuesday,
				time.Wednesday,
				time.Thursday,
				time.Friday,
			),
			EnablePrecision(),
		)
		require.NoError(t, err)

		next := schedule.Next(current)

		// Next interval would be 18:15, past endTime (18:10)
		// Weekend not allowed, so should go to Monday 9:00 AM
		expected := parseTime(t, "2024-03-18 09:00:00") // Monday 9:00 AM
		assert.Equal(t, expected, next,
			"Should skip weekend and go to Monday startTime when Friday exceeds time window")
	})

	t.Run("precision vs non-precision at time boundary", func(t *testing.T) {
		// Compare precision modes at exact boundary
		current := parseTime(t, "2024-03-11 18:10:00") // Monday 18:10 (exact endTime)

		startTime := time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC)
		endTime := time.Date(2000, 1, 1, 18, 10, 0, 0, time.UTC)

		// Test precision mode
		precisionSchedule, err := New(
			15,
			Minute,
			SetStartTime(&startTime),
			SetEndTime(&endTime),
			EnablePrecision(),
		)
		require.NoError(t, err)

		// Test non-precision mode
		nonPrecisionSchedule, err := New(
			15,
			Minute,
			SetStartTime(&startTime),
			SetEndTime(&endTime),
			DisablePrecision(),
		)
		require.NoError(t, err)

		precisionNext := precisionSchedule.Next(current)
		nonPrecisionNext := nonPrecisionSchedule.Next(current)

		// Both should go to next day at startTime since we're at exact endTime
		expected := parseTime(t, "2024-03-12 08:00:00") // Tuesday 8:00 AM

		assert.Equal(t, expected, precisionNext,
			"Precision mode should move to next day startTime when at exact endTime")
		assert.Equal(t, expected, nonPrecisionNext,
			"Non-precision mode should move to next day startTime when at exact endTime")
	})
}

func TestSchedule_ManualNextRun(t *testing.T) {
	// Simple test: 10-second intervals, pause from 10 AM to 3 PM
	current := parseTime(t, "2024-03-11 10:00:00")
	pauseUntil := parseTime(t, "2024-03-11 15:00:00")

	schedule, err := New(10, Second, SetNextRun(&pauseUntil))
	require.NoError(t, err)

	next := schedule.Next(current)
	assert.Equal(t, pauseUntil, next, "Should return manually set next run time")
}
