package rest_test

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/cors"
	"github.com/iqhive/runtime.link/api/rest"
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

// openStream starts a server-side stream of values, closing after them.
func openStream(values ...string) func(context.Context) (<-chan string, error) {
	return func(ctx context.Context) (<-chan string, error) {
		ch := make(chan string)
		go func() {
			defer close(ch)
			for _, value := range values {
				select {
				case ch <- value:
				case <-ctx.Done():
					return
				}
			}
		}()
		return ch, nil
	}
}

// dialSocket performs a handshake over a raw connection and returns the
// connection, a reader past the response headers, and the status line.
func dialSocket(t *testing.T, addr, connection string, headers ...string) (net.Conn, *bufio.Reader, string) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	conn.SetDeadline(time.Now().Add(20 * time.Second))
	key := base64.StdEncoding.EncodeToString(make([]byte, 16))
	fmt.Fprintf(conn, "GET /stream HTTP/1.1\r\nHost: %s\r\nConnection: %s\r\nUpgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: %s\r\n", addr, connection, key)
	for i := 0; i+1 < len(headers); i += 2 {
		fmt.Fprintf(conn, "%s: %s\r\n", headers[i], headers[i+1])
	}
	fmt.Fprint(conn, "\r\n")
	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			return conn, reader, status
		}
	}
}

func socketServer(t *testing.T, auth api.Auth[*http.Request], impl streamAPI) string {
	t.Helper()
	h, err := rest.Handler(auth, impl)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://")
}

func TestWebsocketDisconnectReleasesTheHandler(t *testing.T) {
	released := make(chan struct{})
	started := make(chan struct{}, 1)
	addr := socketServer(t, nil, streamAPI{
		Stream: func(ctx context.Context) (<-chan string, error) {
			ch := make(chan string)
			go func() {
				started <- struct{}{}
				<-ctx.Done()
				close(released)
			}()
			return ch, nil
		},
	})
	conn, _, _ := dialSocket(t, addr, "Upgrade")
	<-started
	conn.Close()
	select {
	case <-released:
	case <-time.After(5 * time.Second):
		t.Errorf("the handler's context outlived the client")
	}
}

func TestWebsocketAnswersACloseFrame(t *testing.T) {
	released := make(chan struct{})
	addr := socketServer(t, nil, streamAPI{
		Stream: func(ctx context.Context) (<-chan string, error) {
			go func() {
				<-ctx.Done()
				close(released)
			}()
			return make(chan string), nil
		},
	})
	conn, reader, _ := dialSocket(t, addr, "Upgrade")
	// A masked close frame with status 1000 and a zero mask.
	conn.Write([]byte{0x88, 0x82, 0, 0, 0, 0, 0x03, 0xE8})
	opcode, payload := readFrame(t, reader)
	if opcode != 0x8 || string(payload) != "\x03\xE8" {
		t.Errorf("reply to a close frame: opcode %#x payload %q", opcode, payload)
	}
	select {
	case <-released:
	case <-time.After(5 * time.Second):
		t.Errorf("the handler's context outlived the close handshake")
	}
}

func TestWebsocketPongEchoesThePing(t *testing.T) {
	addr := socketServer(t, nil, streamAPI{
		Stream: func(ctx context.Context) (<-chan string, error) { return make(chan string), nil },
	})
	conn, reader, _ := dialSocket(t, addr, "Upgrade")
	// A masked ping: "hi" under the mask 1,2,3,4.
	conn.Write([]byte{0x89, 0x82, 1, 2, 3, 4, 'h' ^ 1, 'i' ^ 2})
	if opcode, payload := readFrame(t, reader); opcode != 0xA || string(payload) != "hi" {
		t.Errorf("pong: opcode %#x payload %q", opcode, payload)
	}
}

func TestWebsocketHandshakeTokensIgnoreCase(t *testing.T) {
	addr := socketServer(t, nil, streamAPI{Stream: openStream()})
	for _, connection := range []string{"Upgrade", "upgrade", "UPGRADE", "keep-alive, Upgrade", "Keep-Alive,upgrade"} {
		if _, _, status := dialSocket(t, addr, connection); !strings.Contains(status, "101") {
			t.Errorf("Connection: %s refused: %q", connection, status)
		}
	}
	if _, _, status := dialSocket(t, addr, "notupgrade"); strings.Contains(status, "101") {
		t.Errorf("a token merely containing upgrade was accepted")
	}
}

func TestWebsocketOrigin(t *testing.T) {
	var opened atomic.Int64
	impl := streamAPI{
		Stream: func(ctx context.Context) (<-chan string, error) {
			opened.Add(1)
			return openStream()(ctx)
		},
	}
	open := socketServer(t, nil, impl)
	configured := socketServer(t, originAuth{allow: "https://app.example"}, impl)
	wildcard := socketServer(t, originAuth{allow: "*"}, impl)
	for _, tc := range []struct {
		name, addr, origin string
		allowed            bool
	}{
		{"no origin", open, "", true},
		{"same origin", open, "http://" + open, true},
		{"same origin, https", open, "https://" + open, true},
		{"same origin ignoring case", open, "http://" + strings.ToUpper(open), true},
		{"another site", open, "https://evil.example", false},
		{"another port", open, "http://127.0.0.1:1", false},
		{"opaque origin", open, "null", false},
		{"not a url", open, "://", false},
		{"allowed by cors", configured, "https://app.example", true},
		{"allowed by cors ignoring case", configured, "https://APP.example", true},
		{"same origin beside cors", configured, "http://" + configured, true},
		{"refused by cors", configured, "https://evil.example", false},
		{"wildcard cors", wildcard, "https://anything.example", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := opened.Load()
			headers := []string{}
			if tc.origin != "" {
				headers = append(headers, "Origin", tc.origin)
			}
			_, _, status := dialSocket(t, tc.addr, "Upgrade", headers...)
			if strings.Contains(status, "101") != tc.allowed {
				t.Errorf("status %q", status)
			}
			if !tc.allowed {
				if !strings.Contains(status, "403") {
					t.Errorf("status %q, want 403", status)
				}
				if opened.Load() != before {
					t.Errorf("the function ran for a refused origin")
				}
			}
		})
	}
}

// originAuth configures a single allowed origin.
type originAuth struct {
	allow string
}

func (originAuth) Authenticate(ctx context.Context, r *http.Request, fn api.Function) (context.Context, error) {
	return ctx, nil
}
func (originAuth) Authorize(context.Context, *http.Request, api.Function, []reflect.Value) error {
	return nil
}
func (originAuth) Redact(ctx context.Context, err error) error { return err }
func (a originAuth) CrossOriginResourceSharing(*http.Request, api.Function) cors.AccessControl {
	return cors.AccessControl{AllowOrigin: a.allow}
}

func TestEventStreamDeliversEveryValue(t *testing.T) {
	h, err := rest.Handler(nil, streamAPI{Stream: openStream("a", "b", "c")})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL+"/stream", nil)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if n := strings.Count(string(body), "data: "); n != 3 {
		t.Errorf("%d of 3 events: %q", n, body)
	}
}

func TestEventStreamKeepsConfiguredOrigin(t *testing.T) {
	h, err := rest.Handler(corsRecorder{}, streamAPI{Stream: openStream("x")})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL+"/stream", nil)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Origin", "https://evil.example")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Errorf("Access-Control-Allow-Origin %q", got)
	}
	open, err := rest.Handler(nil, streamAPI{Stream: openStream("x")})
	if err != nil {
		t.Fatal(err)
	}
	unconfigured := httptest.NewServer(open)
	defer unconfigured.Close()
	req, _ = http.NewRequest("GET", unconfigured.URL+"/stream", nil)
	req.Header.Set("Accept", "text/event-stream")
	resp, err = unconfigured.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin %q with nothing configured", got)
	}
}

func TestClientReceivesAChannelResult(t *testing.T) {
	h, err := rest.Handler(nil, streamAPI{Stream: openStream("a", "b", "c")})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[streamAPI](rest.API, srv.URL, srv.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ch, err := client.Stream(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for value := range ch {
		got = append(got, value)
	}
	if strings.Join(got, "") != "abc" {
		t.Errorf("received %q", got)
	}
}

func TestClientChannelStopsWithItsContext(t *testing.T) {
	h, err := rest.Handler(nil, streamAPI{
		Stream: func(ctx context.Context) (<-chan string, error) {
			ch := make(chan string)
			go func() {
				defer close(ch)
				for {
					select {
					case ch <- "tick":
					case <-ctx.Done():
						return
					}
				}
			}()
			return ch, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[streamAPI](rest.API, srv.URL, srv.Client())
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := client.Stream(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := <-ch; got != "tick" {
		t.Fatalf("first value %q", got)
	}
	cancel()
	timeout := time.After(10 * time.Second)
	for open := true; open; {
		select {
		case _, open = <-ch:
		case <-timeout:
			t.Fatal("the channel stayed open after its context ended")
		}
	}
}

func TestClientChannelReportsARefusal(t *testing.T) {
	h, err := rest.Handler(nil, streamAPI{
		Stream: func(ctx context.Context) (<-chan string, error) { return nil, errors.New("no stream for you") },
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[streamAPI](rest.API, srv.URL, srv.Client())
	if _, err := client.Stream(context.Background()); err == nil || !strings.Contains(err.Error(), "no stream for you") {
		t.Errorf("client.Stream: %v", err)
	}
}

// acceptSocket hijacks w and completes the server side of a handshake.
func acceptSocket(w http.ResponseWriter, r *http.Request) (net.Conn, *bufio.ReadWriter, error) {
	conn, brw, err := http.NewResponseController(w).Hijack()
	if err != nil {
		return nil, nil, err
	}
	hash := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	fmt.Fprintf(brw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n",
		base64.StdEncoding.EncodeToString(hash[:]))
	return conn, brw, brw.Flush()
}

func TestClientReceivesOverAWebsocket(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, brw, err := acceptSocket(w, r)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		brw.Write([]byte{0x81, 3, '"', 'a', '"'})
		brw.Write([]byte{0x88, 0})
		brw.Flush()
	}))
	defer srv.Close()
	client := api.Import[streamAPI](rest.API, srv.URL, srv.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ch, err := client.Stream(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for value := range ch {
		got = append(got, value)
	}
	if strings.Join(got, "") != "a" {
		t.Errorf("received %q", got)
	}
}

func TestClientSendsOverAWebsocket(t *testing.T) {
	type API struct {
		api.Specification
		Send func(context.Context) (chan<- string, error) `rest:"GET /send"`
	}
	received := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, brw, err := acceptSocket(w, r)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		var header [6]byte
		if _, err := io.ReadFull(brw, header[:]); err != nil {
			t.Error(err)
			return
		}
		if header[0] != 0x81 || header[1]&0x80 == 0 {
			t.Errorf("header %x: want a masked text frame", header)
		}
		payload := make([]byte, header[1]&0x7F)
		if _, err := io.ReadFull(brw, payload); err != nil {
			t.Error(err)
			return
		}
		for i := range payload {
			payload[i] ^= header[2+i%4]
		}
		received <- payload
	}))
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ch, err := client.Send(ctx)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case ch <- "hello":
	case <-ctx.Done():
		t.Fatal("nobody took the value")
	}
	select {
	case payload := <-received:
		if string(payload) != `"hello"` {
			t.Errorf("payload %q", payload)
		}
	case <-ctx.Done():
		t.Fatal("no frame arrived")
	}
}
