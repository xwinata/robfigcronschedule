package robfigcronschedule

// New creates a new Schedule with the given options.
// Returns an error if the configuration is invalid.
//
// The schedule is enabled by default with no time constraints.
// You must set an interval and intervalTimeUnit for meaningful scheduling.
func New(opts ...scheduleOption) (*Schedule, error) {
	schedule := Schedule{
		enabled:   true,
		precision: true,
	}

	for _, opt := range opts {
		opt(&schedule)
	}

	if err := validate(&schedule); err != nil {
		return nil, err
	}

	return &schedule, nil
}
