package util

import (
	"fmt"
)

type StringSliceFlag []string

func (f *StringSliceFlag) String() string {
	return fmt.Sprintf("%v", *f)
}

func (f *StringSliceFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}
