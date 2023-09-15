package cpu

type Pointer struct {
	addr uintptr
	free func()
}
