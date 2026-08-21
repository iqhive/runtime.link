package rest_test

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"runtime.link/api"
	"runtime.link/api/rest"
)

type streamAPI struct {
	api.Specification

	Stream func(context.Context) (<-chan string, error) `rest:"GET /stream"`
}

// readFrame reads a single unmasked server frame, returning the opcode and
// payload.
func readFrame(t *testing.T, r io.Reader) (byte, []byte) {
	t.Helper()
	var control [2]byte
	if _, err := io.ReadFull(r, control[:]); err != nil {
		t.Fatalf("read frame header: %v", err)
	}
	size := uint64(control[1] & 0b01111111)
	if control[1]&0b10000000 != 0 {
		t.Fatalf("server frames must not be masked")
	}
	switch size {
	case 126:
		var buf [2]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			t.Fatalf("read extended length: %v", err)
		}
		size = uint64(binary.BigEndian.Uint16(buf[:]))
	case 127:
		var buf [8]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			t.Fatalf("read extended length: %v", err)
		}
		size = binary.BigEndian.Uint64(buf[:])
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		t.Fatalf("read payload: %v", err)
	}
	return control[0] & 0b00001111, payload
}

// TestWebsocketFraming round-trips payloads across all three frame length
// encodings (7-bit, 16-bit and 64-bit) through the websocket server.
func TestWebsocketFraming(t *testing.T) {
	messages := []string{
		strings.Repeat("a", 50),    // 7-bit length
		strings.Repeat("b", 300),   // 16-bit length
		strings.Repeat("c", 70000), // 64-bit length
	}
	impl := streamAPI{
		Stream: func(ctx context.Context) (<-chan string, error) {
			ch := make(chan string)
			go func() {
				defer close(ch)
				for _, msg := range messages {
					select {
					case ch <- msg:
					case <-ctx.Done():
						return
					}
				}
			}()
			return ch, nil
		},
	}
	handler, err := rest.Handler(nil, impl)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handler)
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	key := base64.StdEncoding.EncodeToString(make([]byte, 16))
	fmt.Fprintf(conn, "GET /stream HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: %s\r\n\r\n", addr, key)

	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(status, "101") {
		t.Fatalf("expected 101 upgrade, got %q (%v)", status, err)
	}
	for { // skip response headers.
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}

	for i, want := range messages {
		opcode, payload := readFrame(t, reader)
		if opcode != 0x1 {
			t.Fatalf("frame %d: expected text opcode, got %#x", i, opcode)
		}
		var got string
		if err := json.Unmarshal(payload, &got); err != nil {
			t.Fatalf("frame %d: invalid JSON payload: %v", i, err)
		}
		if got != want {
			t.Fatalf("frame %d: got %d bytes, want %d bytes", i, len(got), len(want))
		}
	}
	if opcode, _ := readFrame(t, reader); opcode != 0x8 {
		t.Fatalf("expected close frame, got opcode %#x", opcode)
	}
}
