package util

func Ptr[T any](v T) *T {
	return &v
}

func Empty[T any]() T {
	var zero T
	return zero
}

func As[T any](v any, callback func(T)) {
	if val, ok := v.(T); ok {
		callback(val)
	}
}
