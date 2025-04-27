package util

func Ptr[T any](v T) *T {
	return &v
}

func Empty[T any]() T {
	var zero T
	return zero
}
