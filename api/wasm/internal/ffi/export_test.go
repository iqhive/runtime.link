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

// HeldBytes counts what the open handles hold of the host.
func (ffi *API) HeldBytes() int64 {
	ffi.refs.mu.Lock()
	defer ffi.refs.mu.Unlock()
	return ffi.refs.held
}

// FewerCaps lowers how many handles one bridge may have open at once, and how
// many bytes of the host they may hold, and returns what they were, so that a
// test can raise them back.
func FewerCaps(handles, held int64) (int64, int64) {
	whatTheyWere := [2]int64{maxOpenHandles, maxHeldBytes}
	maxOpenHandles, maxHeldBytes = handles, held
	return whatTheyWere[0], whatTheyWere[1]
}
