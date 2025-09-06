# robfig/cron Schedule Extension ![coverage](https://raw.githubusercontent.com/xwinata/robfigcronschedule/refs/heads/badges/.badges/main/coverage.svg)

A flexible and powerful scheduling library that extends the [robfig/cron](https://github.com/robfig/cron) package with advanced features like time windows, start dates, precision control, and comprehensive interval options.

## Features

- 🕒 **Time Windows**: Define daily start/end times for job execution
- 📅 **Start Dates**: Schedule jobs to begin at specific dates
- ⚡ **Precision Control**: Choose between strict interval timing or window-aligned scheduling
- 🔧 **Flexible Intervals**: Support for seconds, minutes, hours, days, weeks, months, and years
- 🎣 **Execution Hooks**: Before/after execution callbacks for monitoring and logging
- 🛡️ **Robust Validation**: Comprehensive configuration validation with helpful error messages
- 🌍 **Timezone Aware**: Proper timezone handling for global applications

## When to Use This Library vs Cron Expressions

**Use this library for:**
- Interval-based scheduling (every N minutes/hours)
- Time-window restrictions (9 AM - 5 PM only)  
- Start date delays (begin next Monday)
- Dynamic schedule updates

**Use cron expressions for:**
- Complex patterns ("every 15th of month at 2:30 AM")
- Timezone handling (set on cron instance)
- Traditional cron-like scheduling

## Design Philosophy

This library is designed for **one schedule per cron job**. Each schedule instance should be used with a single cron job entry. For multiple scheduling patterns, create separate schedule instances.

## Installation

```bash
go get github.com/xwinata/robfigcronschedule
```

## Quick Start

```go
package main

import (
    "fmt"
    "log"
    "time"
    
    "github.com/robfig/cron/v3"
    rcs "github.com/xwinata/robfigcronschedule"
)

func main() {
    // Create a schedule that runs every 30 minutes
    schedule, err := rcs.New(30, rcs.Minute)
    if err != nil {
        log.Fatal(err)
    }
    
    // Use with robfig/cron
    c := cron.New()
    c.Schedule(schedule, cron.FuncJob(func() {
        fmt.Println("Job executed at:", time.Now())
    }))
    
    c.Start()
    defer c.Stop()
    
    // Keep the program running
    select {}
}
```

## Configuration Options

### Basic Interval Configuration

```go
// Every 5 minutes
schedule, _ := rcs.New(5, rcs.Minute)

// Every 2 hours
schedule, _ := rcs.New(2, rcs.Hour)

// Every day
schedule, _ := rcs.New(1, rcs.Day)
```

### Time Window Configuration

```go
// Run only during business hours (9 AM - 5 PM)
schedule, _ := rcs.New(30, rcs.Minute,
    rcs.SetStartTime(time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)),
    rcs.SetEndTime(time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC)),
)

// Run from 10 PM until end of day
schedule, _ := rcs.New(1, rcs.Hour,
    rcs.SetStartTime(time.Date(2000, 1, 1, 22, 0, 0, 0, time.UTC)),
    // endTime defaults to 23:59:59 if not set
)
```

### Start Date Configuration

```go
// Start the schedule next Monday
nextMonday := getNextMonday()
schedule, _ := rcs.New(1, rcs.Day,
    rcs.SetStartDate(nextMonday)
)

// Start at a specific date and time
launchTime := time.Date(2024, 12, 25, 0, 0, 0, 0, time.UTC)
schedule, _ := rcs.New(4, rcs.Hour,
    rcs.SetStartDate(launchTime),
    rcs.SetStartTime(time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC)),
)
```
### Weekday Filtering

```go
// Run only on weekdays (Monday-Friday)
schedule, _ := rcs.New(30, rcs.Minute,
    rcs.SetAllowedWeekdays(time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday),
)

// Run only on weekends
schedule, _ := rcs.New(2, rcs.Hour,
    rcs.SetAllowedWeekdays(time.Saturday, time.Sunday),
)

// Run only on specific days (e.g., Monday and Wednesday)
schedule, _ := rcs.New(1, rcs.Day,
    rcs.SetStartTime(time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)),
    rcs.SetAllowedWeekdays(time.Monday, time.Wednesday),
)
```
Note: Weekday filtering with multi-week/month/year intervals may produce unexpected results and will return a validation error.

## Precision Mode

The library supports two scheduling modes:

### Precision Mode (Default: `true`)

Calculates intervals strictly from the current time. If the next interval falls outside the daily time window, it moves to the next allowed day.

```go
// 30-minute intervals from current time
schedule, _ := rcs.New(30, rcs.Minute,
    rcs.SetStartTime(time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)),   // 9 AM
    rcs.SetEndTime(time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC)),    // 5 PM  
    rcs.EnablePrecision(), // Default
)

// Current time: 10:45 AM → Next run: 11:15 AM
// Current time: 4:50 PM  → Next run: 9:00 AM (next day)
```

### Non-Precision Mode

Rounds up from the start time using intervals. Ensures no time slots are missed within the daily window.

```go
// Non-precision mode - rounds up from start time
schedule, _ := rcs.New(30, rcs.Minute,
    rcs.SetStartTime(time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)),
    rcs.SetEndTime(time.Date(2000, 1, 1, 17, 0, 0, 0, time.UTC)),
    rcs.DisablePrecision(),
)

// Current time: 10:45 AM → Next run: 11:00 AM (aligned to 9:00, 9:30, 10:00, 10:30, 11:00...)
// Current time: 4:50 PM  → Next run: 9:00 AM (next day)
```

## Practical Use Cases

### 1. Business Hours Data Processing

```go
// Process data every 15 minutes during business hours, Monday-Friday  
schedule, err := rcs.New(15, rcs.Minute,
    rcs.SetStartTime(time.Date(2000, 1, 1, 8, 30, 0, 0, time.Local)),
    rcs.SetEndTime(time.Date(2000, 1, 1, 17, 30, 0, 0, time.Local)),
    rcs.SetAfterNextFunc(func(next *time.Time) {
        log.Printf("Next data processing scheduled for: %v", next)
    }),
)

c := cron.New()
c.Schedule(schedule, cron.FuncJob(processBusinessData))
c.Start()
```

### 2. Maintenance Window

```go
// Run maintenance every Sunday at 2 AM
schedule, err := rcs.New(7, rcs.Day, // Every 7 days
    rcs.SetStartTime(time.Date(2000, 1, 1, 2, 0, 0, 0, time.UTC)),
    rcs.SetBeforeNextFunc(func(s *Schedule) {
        log.Println("Preparing for maintenance...")
    }),
    rcs.SetAfterNextFunc(func(next *time.Time) {
        log.Printf("Next maintenance scheduled for: %v", next.Format("2006-01-02 15:04:05"))
    }),
)

c := cron.New()
c.Schedule(schedule, cron.FuncJob(performMaintenance))
c.Start()
```

### 3. Backup Schedule

```go
// Daily backups at 11 PM, weekdays only
schedule, err := rcs.New(1, rcs.Day,
    rcs.SetStartTime(time.Date(2000, 1, 1, 23, 0, 0, 0, time.Local)),
    rcs.SetAllowedWeekdays(time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday),
)

c := cron.New()
c.Schedule(schedule, cron.FuncJob(performBackup))
c.Start()
```

### 4. Rate-Limited API Calls

```go
// Call external API every 30 seconds, but only during off-peak hours
schedule, err := rcs.New(30, rcs.Second,
    rcs.SetStartTime(time.Date(2000, 1, 1, 1, 0, 0, 0, time.UTC)),  // 1 AM
    rcs.SetEndTime(time.Date(2000, 1, 1, 6, 0, 0, 0, time.UTC)),    // 6 AM
    rcs.EnablePrecision(),
)

c := cron.New()
c.Schedule(schedule, cron.FuncJob(callExternalAPI))
c.Start()
```

### 5. Gradual Rollout

```go
// Start a feature rollout next Monday, then run every hour
rolloutStart := getNextMonday()
schedule, err := rcs.New(1, rcs.Hour,
    rcs.SetStartDate(rolloutStart),
    rcs.SetAfterNextFunc(func(next *time.Time) {
        metrics.RecordScheduledRollout(next)
    }),
)

c := cron.New()
c.Schedule(schedule, cron.FuncJob(performRolloutStep))
c.Start()
```

### 6. Monitoring with Hooks

```go
// Health check every 5 minutes with comprehensive monitoring
schedule, err := rcs.New(5, rcs.Minute,
    rcs.SetBeforeNextFunc(func(s *Schedule) {
        log.Println("Starting health check cycle...")
        metrics.IncrementCounter("health_checks_started")
    }),
    rcs.SetAfterNextFunc(func(next *time.Time) {
        log.Printf("Health check completed. Next check: %v", next)
        metrics.RecordGauge("next_health_check_seconds", 
            time.Until(*next).Seconds())
    }),
)

c := cron.New()
c.Schedule(schedule, cron.FuncJob(performHealthCheck))
c.Start()
```

### 7. Seasonal Schedule

```go
// Different intervals for peak vs off-peak seasons
var schedule *rcs.Schedule

if isPeakSeason() {
    // Peak season: every 10 minutes during business hours
    schedule, _ = rcs.New(10, rcs.Minute,
        rcs.SetStartTime(time.Date(2000, 1, 1, 8, 0, 0, 0, time.Local)),
        rcs.SetEndTime(time.Date(2000, 1, 1, 20, 0, 0, 0, time.Local)),
    )
} else {
    // Off-peak: every 30 minutes, extended hours
    schedule, _ = rcs.New(30, rcs.Minute,
        rcs.SetStartTime(time.Date(2000, 1, 1, 6, 0, 0, 0, time.Local)),
        rcs.SetEndTime(time.Date(2000, 1, 1, 22, 0, 0, 0, time.Local)),
    )
}

c := cron.New()
c.Schedule(schedule, cron.FuncJob(processSeasonalData))
c.Start()
```

## Dynamic Schedule Updates

```go
// Example: Update schedule based on database configuration
func createDynamicSchedule() *rcs.Schedule {
    schedule, _ := rcs.New(5, rcs.Second, // Default fallback
        rcs.SetBeforeNextFunc(func(s *Schedule) {
            // Query database for current configuration
            config := fetchScheduleConfigFromDB()
            
            // Apply updates if configuration changed
            var updates []rcs.ScheduleOption
            if config.IntervalSeconds != s.GetInterval() {
                updates = append(updates, rcs.SetInterval(config.IntervalSeconds))
            }
            if config.Enabled != s.IsEnabled() {
                if config.Enabled {
                    updates = append(updates, rcs.Enable())
                } else {
                    updates = append(updates, rcs.Disable())
                }
            }
            
            if len(updates) > 0 {
                s.Set(updates...)
            }
        }),
    )
    return schedule
}

// Example: Feature flag integration
schedule, _ := rcs.New(30, rcs.Minute,
    rcs.SetBeforeNextFunc(func(s *Schedule) {
        // Check feature flags before each execution
        if featureFlags.IsDisabled("background_processing") {
            s.Set(rcs.Disable())
            return
        }
        
        // Dynamic interval based on load
        if systemLoad.IsHigh() {
            s.Set(rcs.SetInterval(60)) // Slow down during high load
        } else {
            s.Set(rcs.SetInterval(30)) // Normal interval
        }
    }),
)
```

## Error Handling and Validation

```go
// The library provides comprehensive validation
schedule, err := rcs.New(-5, rcs.Hour, // Invalid: negative interval
    rcs.SetStartTime(time.Date(2000, 1, 1, 10, 0, 0, 0, time.UTC)),
    rcs.SetEndTime(time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)), // Invalid: end before start
)

if err != nil {
    fmt.Printf("Configuration error: %v\n", err)
    // Output: Configuration error: interval cannot be less than 1
}

// Validation also occurs during updates
err = schedule.Set(rcs.SetInterval(0))
if err != nil {
    fmt.Printf("Update error: %v\n", err)
    // Schedule remains unchanged
}
```

## API Reference

### Types

```go
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
```

### Constructor

```go
func New(interval int, intervalTimeUnit IntervalTimeUnit, opts ...scheduleOption) (*Schedule, error)
```

### Configuration Options

```go
func SetStartTime(t time.Time) scheduleOption
func SetEndTime(t time.Time) scheduleOption  
func SetStartDate(t time.Time) scheduleOption
func SetAllowedWeekdays(weekdays ...time.Weekday) scheduleOption
func SetBeforeNextFunc(f func()) scheduleOption
func SetAfterNextFunc(f func(next *time.Time)) scheduleOption
func Enable() scheduleOption
func Disable() scheduleOption
func EnablePrecision() scheduleOption  // Default
func DisablePrecision() scheduleOption

// Set() method updates only:
func SetInterval(i int) scheduleOption         // For updating existing schedules
func SetIntervalTimeUnit(i IntervalTimeUnit) scheduleOption  // For updating existing schedules
```

### Methods

```go
func (s *Schedule) Next(t time.Time) time.Time  // robfig/cron.Schedule interface
func (s *Schedule) Set(opts ...scheduleOption) error
```

## Error Handling

The library provides comprehensive validation with clear error messages:

```go
// Invalid interval - now caught at construction
schedule, err := rcs.New(-5, rcs.Minute) // Invalid: negative interval  
if err != nil {
    fmt.Printf("Configuration error: %v\n", err)
    // Output: Configuration error: invalid interval. interval cannot be less than 1
}

// Invalid time window
schedule, err := rcs.New(30, rcs.Minute,
    rcs.SetStartTime(time.Date(2000, 1, 1, 10, 0, 0, 0, time.UTC)),
    rcs.SetEndTime(time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC)),
)
if err != nil {
    // err: "invalid time window. start time must be before end time"  
}

// Multi-interval with weekdays
schedule, err := rcs.New(2, rcs.Hour,
    rcs.SetIntervalTimeUnit(rcs.Week),
    rcs.SetAllowedWeekdays(time.Monday),
)
if err != nil {
    // err: "multi weeks/months/years intervals with weekday restrictions may produce unexpected results"
}

// Configuration updates are validated and rolled back on error
err = schedule.Set(rcs.SetInterval(0))
if err != nil {
    // Schedule remains unchanged
    log.Printf("Update failed: %v", err)
}
```

## Contributing

We welcome contributions! Please create issue and-or pull request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built on top of the excellent [robfig/cron](https://github.com/robfig/cron) library
- Inspired by enterprise scheduling requirements and real-world use cases