package ffi

// LiveHandles counts the handles that are still open.
func (ffi *API) LiveHandles() int {
	var live int
	ffi.refs.m.Range(func(_, _ any) bool {
		live++
		return true
	})
	return live
}
