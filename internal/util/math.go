package util

import "golang.org/x/exp/constraints"

func Clamp[T constraints.Ordered](value, min, max T) T {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func RangeOrElse[T constraints.Ordered](value, min, max, elseValue T) T {
	if value < min || value > max {
		return elseValue
	}
	return value
}
