package util

func Ptr[T any](v T) *T {
	return &v
}

func Empty[T any]() T {
	var zero T
	return zero
}

func Ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}
