package models

// IPAddr returns a pointer to the string if it's not empty, otherwise returns nil.
// This is useful for creating optional string fields in Go structs that correspond to nullable SQL columns.
func IPAddr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
