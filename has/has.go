package has

func New[T any](val T) *T { return &val }

type Minimum[T any] interface {
	Min() T
}

type Maximum[T any] interface {
	Max() T
}

type Documentation [0]struct{}

type Validation interface {
	Validate() error
}
