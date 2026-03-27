package core

// SharedDistribution provides utilities for calculating shared distribution of bytes
// among multiple users/pinners, following anonymous distribution logic.

// CalculateSharedBytes calculates the bytes per user with configurable precision.
//
// This implements the party sharing algorithm for anonymous distribution where
// costs are divided among users who make content available to anonymous users.
//
// Precision semantics:
//   0  -> floor to whole byte, minimum 0
//   >0 -> ceil to whole byte, minimum 1 when totalBytes > 0
//
//Parameters:
//   - totalBytes: Total bytes to be distributed
//   - userCount: Number of users to distribute among
//   - precision: Precision level (0 = floor, >0 = ceil)
//
// Returns the number of bytes each user should be charged.
func CalculateSharedBytes(totalBytes uint64, userCount uint, precision int) uint64 {
	if userCount == 0 {
		return 0
	}

	var roundedBytes uint64
	if precision > 0 {
		// Ceiling division without float conversion
		roundedBytes = totalBytes / uint64(userCount)
		if totalBytes % uint64(userCount) != 0 {
			roundedBytes++
		}
		// Ensure minimum of 1 byte when totalBytes > 0
		if roundedBytes == 0 && totalBytes > 0 {
			roundedBytes = 1
		}
	} else {
		// Floor division
		roundedBytes = totalBytes / uint64(userCount)
	}

	return roundedBytes
}
