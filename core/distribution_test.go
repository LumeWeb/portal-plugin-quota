package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCalculateSharedBytes_ExactDivision tests exact division scenarios
func TestCalculateSharedBytes_ExactDivision(t *testing.T) {
	tests := []struct {
		name         string
		totalBytes   uint64
		userCount    uint
		precision    int
		expected     uint64
		description  string
	}{
		{
			name:        "exact division with precision 0",
			totalBytes:  1000,
			userCount:   4,
			precision:   0,
			expected:    250,
			description: "1000 bytes / 4 users = 250 bytes per user",
		},
		{
			name:        "exact division with precision 1",
			totalBytes:  1000,
			userCount:   4,
			precision:   1,
			expected:    250,
			description: "1000 bytes / 4 users = 250 bytes per user, no ceiling needed",
		},
		{
			name:        "exact division with large precision",
			totalBytes:  1000,
			userCount:   4,
			precision:   10,
			expected:    250,
			description: "1000 bytes / 4 users = 250 bytes per user, no ceiling needed",
		},
		{
			name:        "exact division single user",
			totalBytes:  500,
			userCount:   1,
			precision:   0,
			expected:    500,
			description: "500 bytes / 1 user = 500 bytes per user",
		},
		{
			name:        "exact division multiple users",
			totalBytes:  100,
			userCount:   2,
			precision:   0,
			expected:    50,
			description: "100 bytes / 2 users = 50 bytes per user",
		},
		{
			name:        "exact division with zero precision",
			totalBytes:  1000,
			userCount:   10,
			precision:   0,
			expected:    100,
			description: "1000 bytes / 10 users = 100 bytes per user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateSharedBytes(tt.totalBytes, tt.userCount, tt.precision)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

// TestCalculateSharedBytes_FloorDivisionPrecisionZero tests floor division behavior with precision 0
func TestCalculateSharedBytes_FloorDivisionPrecisionZero(t *testing.T) {
	tests := []struct {
		name         string
		totalBytes   uint64
		userCount    uint
		precision    int
		expected     uint64
		description  string
	}{
		{
			name:        "floor division small bytes many users",
			totalBytes:  1,
			userCount:   10,
			precision:   0,
			expected:    0,
			description: "1 byte / 10 users = 0 bytes per user (floored)",
		},
		{
			name:        "floor division uneven distribution",
			totalBytes:  1000,
			userCount:   3,
			precision:   0,
			expected:    333,
			description: "1000 bytes / 3 users = 333.33... floored to 333",
		},
		{
			name:        "floor division smaller than 1 byte per user",
			totalBytes:  5,
			userCount:   10,
			precision:   0,
			expected:    0,
			description: "5 bytes / 10 users = 0.5 bytes floored to 0",
		},
		{
			name:        "floor division just above 1",
			totalBytes:  11,
			userCount:   10,
			precision:   0,
			expected:    1,
			description: "11 bytes / 10 users = 1.1 floored to 1",
		},
		{
			name:        "floor division large numbers",
			totalBytes:  999,
			userCount:   7,
			precision:   0,
			expected:    142,
			description: "999 bytes / 7 users = 142.714... floored to 142",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateSharedBytes(tt.totalBytes, tt.userCount, tt.precision)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

// TestCalculateSharedBytes_CeilDivisionPrecision tests ceiling division behavior with precision > 0
func TestCalculateSharedBytes_CeilDivisionPrecision(t *testing.T) {
	tests := []struct {
		name         string
		totalBytes   uint64
		userCount    uint
		precision    int
		expected     uint64
		description  string
	}{
		{
			name:        "ceil division precision 1",
			totalBytes:  1000,
			userCount:   3,
			precision:   1,
			expected:    334,
			description: "1000 bytes / 3 users = 333.33..., ceiled to 334",
		},
		{
			name:        "ceil division precision 2",
			totalBytes:  1000,
			userCount:   3,
			precision:   2,
			expected:    334,
			description: "1000 bytes / 3 users = 333.33..., ceiled to 334",
		},
		{
			name:        "ceil division precision 10",
			totalBytes:  1000,
			userCount:   3,
			precision:   10,
			expected:    334,
			description: "1000 bytes / 3 users = 333.33..., ceiled to 334",
		},
		{
			name:        "ceil division tiny amount many users",
			totalBytes:  1,
			userCount:   10,
			precision:   2,
			expected:    1,
			description: "1 byte / 10 users = 0.1, but min 1 byte with precision>0",
		},
		{
			name:        "ceil division exact result",
			totalBytes:  100,
			userCount:   4,
			precision:   1,
			expected:    25,
			description: "100 bytes / 4 users = 25, ceiling doesn't change result",
		},
		{
			name:        "ceil division nearly exact with remainder",
			totalBytes:  1001,
			userCount:   4,
			precision:   1,
			expected:    251,
			description: "1001 bytes / 4 users = 250.25, ceiled to 251",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateSharedBytes(tt.totalBytes, tt.userCount, tt.precision)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

// TestCalculateSharedBytes_EdgeCases tests edge cases and boundary conditions
func TestCalculateSharedBytes_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		totalBytes   uint64
		userCount    uint
		precision    int
		expected     uint64
		description  string
	}{
		{
			name:        "zero total bytes",
			totalBytes:  0,
			userCount:   5,
			precision:   0,
			expected:    0,
			description: "0 bytes / 5 users = 0 bytes per user",
		},
		{
			name:        "zero total bytes with precision",
			totalBytes:  0,
			userCount:   5,
			precision:   1,
			expected:    0,
			description: "0 bytes / 5 users = 0 bytes per user (no min for zero total)",
		},
		{
			name:        "zero user count",
			totalBytes:  1000,
			userCount:   0,
			precision:   0,
			expected:    0,
			description: "zero users results in 0 bytes (guard clause)",
		},
		{
			name:        "zero user count with precision",
			totalBytes:  1000,
			userCount:   0,
			precision:   1,
			expected:    0,
			description: "zero users results in 0 bytes (guard clause)",
		},
		{
			name:        "one user any bytes",
			totalBytes:  500,
			userCount:   1,
			precision:   0,
			expected:    500,
			description: "single user gets total bytes",
		},
		{
			name:        "one user with precision",
			totalBytes:  500,
			userCount:   1,
			precision:   1,
			expected:    500,
			description: "single user gets total bytes regardless of precision",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateSharedBytes(tt.totalBytes, tt.userCount, tt.precision)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

// TestCalculateSharedBytes_LargeNumbers tests large value calculations
func TestCalculateSharedBytes_LargeNumbers(t *testing.T) {
	tests := []struct {
		name         string
		totalBytes   uint64
		userCount    uint
		precision    int
		expected     uint64
		description  string
	}{
		{
			name:        "large bytes exact division",
			totalBytes:  1 << 30, // 1GB
			userCount:   1024,
			precision:   0,
			expected:    1048576, // 1MB
			description: "1GB / 1024 users = 1MB per user",
		},
		{
			name:        "large bytes uneven division",
			totalBytes:  1 << 30, // 1GB
			userCount:   3,
			precision:   0,
			expected:    357913941, // ~341MB approx
			description: "1GB / 3 users, floored result",
		},
		{
			name:        "many users small bytes",
			totalBytes:  1000,
			userCount:   100,
			precision:   0,
			expected:    10,
			description: "1000 bytes / 100 users = 10 bytes per user",
		},
		{
			name:        "many users with precision",
			totalBytes:  1000,
			userCount:   100,
			precision:   1,
			expected:    10,
			description: "1000 bytes / 100 users = 10 bytes per user, exact division",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateSharedBytes(tt.totalBytes, tt.userCount, tt.precision)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

// TestCalculateSharedBytes_MinimumByteEnforcement tests the minimum 1 byte enforcement
func TestCalculateSharedBytes_MinimumByteEnforcement(t *testing.T) {
	tests := []struct {
		name         string
		totalBytes   uint64
		userCount    uint
		precision    int
		expected     uint64
		description  string
	}{
		{
			name:        "min byte with tiny total precision 1",
			totalBytes:  1,
			userCount:   1000,
			precision:   1,
			expected:    1,
			description: "1 byte / 1000 users = 0.001, but min 1 with precision>0",
		},
		{
			name:        "min byte with tiny total precision 2",
			totalBytes:  1,
			userCount:   1000,
			precision:   2,
			expected:    1,
			description: "1 byte / 1000 users = 0.001, but min 1 with precision>0",
		},
		{
			name:        "no min byte with precision 0",
			totalBytes:  1,
			userCount:   1000,
			precision:   0,
			expected:    0,
			description: "1 byte / 1000 users = 0.001, floored to 0 with precision=0",
		},
		{
			name:        "min byte with small total many users",
			totalBytes:  10,
			userCount:   20,
			precision:   1,
			expected:    1,
			description: "10 bytes / 20 users = 0.5, but min 1 with precision>0",
		},
		{
			name:        "enough bytes for minimum with precision",
			totalBytes:  100,
			userCount:   10,
			precision:   1,
			expected:    10,
			description: "100 bytes / 10 users = 10, above minimum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateSharedBytes(tt.totalBytes, tt.userCount, tt.precision)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

// TestCalculateSharedBytes_RealWorldScenarios tests realistic usage scenarios
func TestCalculateSharedBytes_RealWorldScenarios(t *testing.T) {
	tests := []struct {
		name         string
		totalBytes   uint64
		userCount    uint
		precision    int
		expected     uint64
		description  string
	}{
		{
			name:        "typical upload many pinners",
			totalBytes:  10000000, // 10MB
			userCount:   5,
			precision:   0,
			expected:    2000000, // 2MB
			description: "10MB upload shared by 5 pinners = 2MB each",
		},
		{
			name:        "large upload few pinners",
			totalBytes:  1000000000, // 1GB
			userCount:   3,
			precision:   0,
			expected:    333333333, // 333MB approx
			description: "1GB upload shared by 3 pinners = 333MB each floored",
		},
		{
			name:        "large upload few pinners with precision",
			totalBytes:  1000000000, // 1GB
			userCount:   3,
			precision:   1,
			expected:    333333334, // 334MB approx
			description: "1GB upload shared by 3 pinners = 334MB each ceiled",
		},
		{
			name:        "small transfer many users exact",
			totalBytes:  10000, // 10KB
			userCount:   10,
			precision:   0,
			expected:    1000, // 1KB
			description: "10KB transfer shared by 10 users = 1KB each",
		},
		{
			name:        "very small transfer many users with precision",
			totalBytes:  100, // 100 bytes
			userCount:   50,
			precision:   1,
			expected:    2,
			description: "100 bytes / 50 users = 2 bytes per user, ceiled",
		},
		{
			name:        "micro transfer many users floor",
			totalBytes:  100, // 100 bytes
			userCount:   50,
			precision:   0,
			expected:    2,
			description: "100 bytes / 50 users = 2 bytes per user (exact division)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateSharedBytes(tt.totalBytes, tt.userCount, tt.precision)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

// TestCalculateSharedBytes_PrecisionLevels verifies different precision values don't affect exact division
func TestCalculateSharedBytes_PrecisionLevels(t *testing.T) {
	// For exact division, all precision levels should give same result
	totalBytes := uint64(1000)
	userCount := uint(10)
	expected := uint64(100)

	precisions := []int {0, 1, 2, 5, 10}
	for _, precision := range precisions {
		t.Run("precision_values", func(t *testing.T) {
			result := CalculateSharedBytes(totalBytes, userCount, precision)
			assert.Equal(t, expected, result, 
				"expected exact division to be same regardless of precision")
		})
	}
}

// TestCalculateSharedBytes_Monotonicity ensures consistent behavior
func TestCalculateSharedBytes_Monotonicity(t *testing.T) {
	t.Run("increasing total bytes", func(t *testing.T) {
		userCount := uint(5)
		precision := 0
		
		prevResult := CalculateSharedBytes(0, userCount, precision)
		for bytes := uint64(1); bytes <= 1000; bytes++ {
			result := CalculateSharedBytes(bytes, userCount, precision)
			assert.True(t, result >= prevResult, 
				"result should not decrease with increasing total bytes")
			prevResult = result
		}
	})
}
