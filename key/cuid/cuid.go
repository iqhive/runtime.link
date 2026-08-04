package cuid

type (
	// V1 CUID identifier to a value of type T
	// https://github.com/paralleldrive/cuid
	V1[T any] string

	// V2 CUID identifier to a value of type T
	// https://github.com/paralleldrive/cuid2
	V2[T any] string
)
