package models

// WindowType defines the time window type for limit enforcement
type WindowType string

const (
	WindowTypeRolling      WindowType = "ROLLING"      // Rolling window: last N seconds
	WindowTypeCalendarDay  WindowType = "DAY"          // Calendar day (resets at midnight)
	WindowTypeCalendarWeek WindowType = "WEEK"         // Calendar week
	WindowTypeCalendarMonth WindowType = "MONTH"       // Calendar month (resets 1st of month)
	WindowTypeCalendarYear WindowType = "YEAR"         // Calendar year (resets Jan 1st)
	WindowTypeLifetime     WindowType = "LIFETIME"     // All-time usage
)

// IsValid returns true if this is a valid window type
func (w WindowType) IsValid() bool {
	return w == WindowTypeRolling ||
		w == WindowTypeCalendarDay ||
		w == WindowTypeCalendarWeek ||
		w == WindowTypeCalendarMonth ||
		w == WindowTypeCalendarYear ||
		w == WindowTypeLifetime
}

// String returns the string representation of the window type
func (w WindowType) String() string {
	return string(w)
}
