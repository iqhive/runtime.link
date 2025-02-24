/*
Package mmm provides memory management utilities for runtime.link.
It implements reference counting and lifetime management for resources,
particularly useful when dealing with FFI and external resources.

# Example Usage

Basic lifetime management:

	type Resource struct {
		data []byte
	}

	// Create a new lifetime
	lt := mmm.NewLifetime()
	defer lt.End()

	// Create a resource with managed lifetime
	resource := mmm.New[Resource](lt, &Resource{
		data: []byte("example"),
	})

	// Resource will be automatically cleaned up when lifetime ends

# Reference Counting

Objects can be shared between different lifetimes:

	lt1 := mmm.NewLifetime()
	lt2 := mmm.NewLifetime()

	obj1 := mmm.New[Resource](lt1, &Resource{...})
	obj2 := mmm.Copy(obj1, lt2)

	// obj1's lifetime ends
	lt1.End()

	// obj2 still valid until lt2 ends
	lt2.End()

# Features

The mmm package provides:
  - Reference counting
  - Resource lifetime management
  - Safe pointer handling
  - Memory leak prevention
  - Integration with runtime.link type system
*/
package mmm
