package mmm_test

import (
	"context"
	"testing"

	"runtime.link/mmm"
)

type API struct {
	NewObject       func(context.Context, string) Object
	GetObjectString func(Object) string
	FreeObject      func(Object)
}

type Object mmm.Pointer[API, Object, int]

func NewAPI() API {
	var objects []string
	var refresh []int

	var api = new(API)
	*api = API{
		NewObject: func(ctx context.Context, s string) Object {
			idx := len(objects) + 1
			if len(refresh) > 0 {
				idx = refresh[len(refresh)-1]
				refresh = refresh[:len(refresh)-1]
				objects[idx-1] = s
			} else {
				objects = append(objects, s)
			}
			return mmm.Make[API, Object](ctx, api, idx)
		},
		GetObjectString: func(obj Object) string {
			return objects[obj.Pointer()-1]
		},
		FreeObject: func(obj Object) {
			idx := obj.Pointer()
			objects[idx-1] = ""
			refresh = append(refresh, int(idx))
			mmm.MarkFree(obj)
		},
	}
	return *api
}

func (obj Object) String() string { return obj.API.GetObjectString(obj) }
func (obj Object) Free()          { obj.API.FreeObject(obj) }

func TestPointer(t *testing.T) {
	ctx := mmm.NewContext(context.Background())
	defer ctx.Free()

	var Objects = NewAPI()

	var obj = Objects.NewObject(ctx, "Hello World")
	if Objects.GetObjectString(obj) != "Hello World" {
		t.Fatal("api.GetObjectString(obj) != \"Hello World\"")
	}
}

func TestMove(t *testing.T) {
	ctx := mmm.NewContext(context.Background())

	var Objects = NewAPI()

	var obj = Objects.NewObject(ctx, "Hello World")

	new := mmm.NewContext(context.Background())

	obj = mmm.Move(obj, new)
	ctx.Free()

	// should still be alive.
	if Objects.GetObjectString(obj) != "Hello World" {
		t.Fatal("api.GetObjectString(obj) != \"Hello World\"")
	}
}

// BenchmarkAllocations should not allocate!
func BenchmarkAllocations(b *testing.B) {
	ctx := mmm.NewContext(context.Background())

	var Objects = NewAPI()
	Objects.NewObject(ctx, "Hello World")
	ctx.Free()

	b.ResetTimer()

	ctx = mmm.NewContext(context.Background())

	for i := 0; i < b.N; i++ {
		Objects.NewObject(ctx, "Hello World")
		ctx.Free()
		ctx = mmm.NewContext(context.Background())
	}
}
