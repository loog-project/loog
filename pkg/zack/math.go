package zack

import "golang.org/x/exp/constraints"

// Clamp constrains a value to be within the specified range [min, max].
// If the value is less than min, it returns min; if greater than max, it returns max.
func Clamp[T constraints.Ordered](value, min, max T) T {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// RangeOrDefault checks if a value is within the specified range [min, max].
// If the value is outside this range, it returns the defaultValue.
func RangeOrDefault[T constraints.Ordered](value, min, max, defaultValue T) T {
	if value < min || value > max {
		return defaultValue
	}
	return value
}
