package ram_test

import (
	"testing"

	"runtime.link/ram"
)

func BenchmarkRAM(t *testing.B) {
	var m = map[string]string{
		"hello": "world",
	}
	t.ReportAllocs()
	for i := 0; i < t.N; i++ {
		if ram.NewBool(true).Bool() != true {
			t.Fatal()
		}
		if ram.MapInMemory(m).Index("hello") != "world" {
			t.Fatal()
		}
	}
}
