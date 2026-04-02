package core

// CheckOptions configures quota check behavior
type CheckOptions struct {
    CreateReservation bool
}

// CheckOption is a function that modifies CheckOptions configuration
type CheckOption func(*CheckOptions)

// WithCreateReservation creates a reservation during quota check
func WithCreateReservation() CheckOption {
    return func(opts *CheckOptions) {
        opts.CreateReservation = true
    }
}

// ParseOptions merges check options
func ParseOptions(opts ...CheckOption) CheckOptions {
    checkOpts := CheckOptions{
        CreateReservation: false,
    }
    for _, opt := range opts {
        opt(&checkOpts)
    }
    return checkOpts
}
