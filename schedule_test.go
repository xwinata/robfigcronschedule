package robfigcronschedule

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_BasicValidation(t *testing.T) {
	tests := []struct {
		name          string
		interval      int
		unit          IntervalTimeUnit
		opts          []scheduleOption
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
			opts: []scheduleOption{
				SetStartTime(time.Date(2000, 1, 1, 10, 0, 0, 0, time.UTC)),
				SetEndTime(time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)),
			},
			expectError: ErrInvalidTimeWindow,
		},
		{
			name:     "empty weekdays restriction",
			interval: 5,
			unit:     Second,
			opts: []scheduleOption{
				SetAllowedWeekdays(), // empty
			},
			expectError: ErrNoDayInWeekdayWindow,
		},
		{
			name:     "multi-week interval with weekday restriction",
			interval: 2,
			unit:     Week,
			opts: []scheduleOption{
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
			// Real-world use case: Process data every 2 seconds during business hours
			schedule, err := New(
				2,
				Second,
				SetStartTime(time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)),
				SetEndTime(time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC)),
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
	// Real-world use case: Daily maintenance every day at 2 AM
	schedule, err := New(1, Day,
		SetStartTime(time.Date(2000, 1, 1, 2, 0, 0, 0, time.UTC)),
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
			var updates []scheduleOption
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

func TestSchedule_RedisConfigUpdate(t *testing.T) {
	// Mock Redis config store
	mockRedis := map[string]interface{}{
		"interval": 2,
		"enabled":  true,
		"version":  1,
	}

	var hookCallCount int

	schedule, err := New(5, Second,
		SetBeforeNextFunc(func(s *Schedule) {
			hookCallCount++

			// Simulate Redis GET operation
			interval, hasInterval := mockRedis["interval"].(int)
			enabled, hasEnabled := mockRedis["enabled"].(bool)

			if !hasInterval || !hasEnabled {
				return // Config not found
			}

			// Apply updates if needed
			var updates []scheduleOption
			if interval != s.interval {
				updates = append(updates, SetInterval(interval))
			}
			if enabled != s.enabled {
				if enabled {
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

	now := time.Now()

	// Initial run - should update interval to 2
	schedule.Next(now)
	assert.Equal(t, 1, hookCallCount)
	assert.Equal(t, 2, schedule.interval)
	assert.True(t, schedule.enabled)

	// Simulate config change in Redis
	mockRedis["interval"] = 4
	mockRedis["enabled"] = false
	mockRedis["version"] = 2

	schedule.Next(now.Add(time.Second))
	assert.Equal(t, 2, hookCallCount)
	assert.Equal(t, 4, schedule.interval)
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

	schedule, err := New(5, Second,
		SetStartDate(futureDate),
		SetStartTime(time.Date(2000, 1, 1, 2, 0, 0, 0, time.UTC)),
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
			opts := []scheduleOption{
				SetStartTime(startTime),
				SetEndTime(endTime),
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
			expected: "2024-03-13 10:00:00",
		},
		{
			name:     "1 week",
			interval: 1,
			unit:     Week,
			expected: "2024-03-18 10:00:00",
		},
		{
			name:     "1 month",
			interval: 1,
			unit:     Month,
			expected: "2024-04-11 10:00:00",
		},
		{
			name:     "1 year",
			interval: 1,
			unit:     Year,
			expected: "2025-03-11 10:00:00",
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
			schedule, err := New(
				1,
				Day,
				SetStartTime(time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)),
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

// Helper functions
func parseTime(t *testing.T, timeStr string) time.Time {
	parsed, err := time.Parse("2006-01-02 15:04:05", timeStr)
	require.NoError(t, err)
	return parsed.UTC()
}
