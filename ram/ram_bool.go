package ram

import "runtime.link/via"

type Bool via.Any[BoolProxy]

func (b Bool) Bool() bool {
	return via.Methods(b).Alive(via.Internal(b))
}

type BoolProxy interface {
	via.API
}

type goMemoryBool struct{}

func (goMemoryBool) Alive(state via.CachedState) bool { return via.Cached[goMemoryBool, bool](state) }

func NewBool[T ~bool](val T) Bool {
	return via.New[Bool, BoolProxy](goMemoryBool{}, via.NewCache[goMemoryBool](val))
}
