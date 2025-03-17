package grpc_test

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"runtime.link/api/grpc"
)

func BenchmarkGRPC(t *testing.B) {
	enc := grpc.NewEncoder(io.Discard)

	t.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			enc.Encode([]int{1, 2, 3})
		}
	})
}

func TestGRPC(t *testing.T) {
	var buf bytes.Buffer
	enc := grpc.NewEncoder(&buf)
	enc.Encode([]int{1, 2, 3})
	fmt.Println(buf.Bytes())
}
