package core

// CheckOptions configures quota check behavior
type CheckOptions struct {
    CreateReservation bool
    IP               string // IP address for the reservation
}

// WithCreateReservation creates a reservation during quota check
func WithCreateReservation(ip string) func(*CheckOptions) {
    return func(opts *CheckOptions) {
        opts.CreateReservation = true
        opts.IP = ip
    }
}

// ParseOptions merges check options
func ParseOptions(opts ...func(*CheckOptions)) CheckOptions {
    checkOpts := CheckOptions{
        CreateReservation: false,
    }
    for _, opt := range opts {
        opt(&checkOpts)
    }
    return checkOpts
}
