package rest

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"
)

// maxFramePayload caps the payload size accepted for a single websocket
// frame. The declared size is attacker-controlled (up to 2^64-1), so it
// must not be passed to make() unchecked; oversized frames close the
// connection.
const maxFramePayload = 16 << 20

// socketWriteTimeout bounds a write to a websocket peer that stopped reading,
// which is otherwise only noticed once the connection's buffers have filled.
const socketWriteTimeout = 30 * time.Second

const (
	sockText  = 0x1
	sockClose = 0x8
	sockPing  = 0x9
	sockPong  = 0xA

	frameFin    = 0b10000000
	frameMasked = 0b10000000
	frameOpcode = 0b00001111
	frameLength = 0b01111111
)

var errFrameTooLarge = errors.New("websocket frame exceeds the payload limit")

// isWebsocketHandshake reports whether r asks to upgrade to a websocket. Header
// tokens are case-insensitive, and a proxy may send Connection: upgrade.
func isWebsocketHandshake(r *http.Request) bool {
	return headerHasToken(r.Header, "Connection", "upgrade") && headerHasToken(r.Header, "Upgrade", "websocket")
}

func headerHasToken(header http.Header, name, token string) bool {
	for _, value := range header.Values(name) {
		for field := range strings.SplitSeq(value, ",") {
			if strings.EqualFold(strings.TrimSpace(field), token) {
				return true
			}
		}
	}
	return false
}

// websocketOriginAllowed guards against cross-site websocket hijacking: a browser
// sends cookies with a websocket from any page, and leaves it to the server to
// refuse the page. No Origin at all means the client is no browser. allowed is
// the CORS AllowOrigin configured for the request. The scheme of the origin is
// not compared, as what terminated TLS in front of this server is unknown here.
func websocketOriginAllowed(r *http.Request, allowed string) bool {
	origin := r.Header.Get("Origin")
	if origin == "" || allowed == "*" || strings.EqualFold(origin, allowed) {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && parsed.Host != "" && strings.EqualFold(parsed.Host, r.Host)
}

// websocketServeHTTP serves a websocket connection, sending and receiving values from the send and recv channels.
func websocketServeHTTP(ctx context.Context, r *http.Request, rw http.ResponseWriter, send, recv reflect.Value) {
	if send.IsValid() && send.Kind() == reflect.Chan && send.Type().ChanDir() == reflect.BothDir {
		http.Error(rw, "bidirectional send channels are not supported for WebSocket communication", http.StatusBadRequest)
		return
	}
	if recv.IsValid() && recv.Kind() == reflect.Chan && recv.Type().ChanDir() == reflect.BothDir {
		http.Error(rw, "bidirectional receive channels are not supported for WebSocket communication", http.StatusBadRequest)
		return
	}
	if r.Method != "GET" {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !isWebsocketHandshake(r) {
		http.Error(rw, "please upgrade your connection to a websocket", http.StatusBadRequest)
		return
	}
	if r.Header.Get("Sec-WebSocket-Version") != "13" {
		rw.Header().Set("Sec-WebSocket-Version", "13")
		http.Error(rw, "unsupported websocket version", http.StatusBadRequest)
		return
	}
	hash := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	rw.Header().Set("Connection", "Upgrade")
	rw.Header().Set("Upgrade", "websocket")
	rw.Header().Set("Sec-WebSocket-Accept", base64.StdEncoding.EncodeToString(hash[:]))
	rw.WriteHeader(101)
	conn, brw, err := http.NewResponseController(rw).Hijack()
	if err != nil {
		return
	}
	write := func(opcode byte, payload []byte) error {
		if err := conn.SetWriteDeadline(time.Now().Add(socketWriteTimeout)); err != nil {
			return err
		}
		_, err := conn.Write(appendFrame(nil, opcode, payload, nil))
		return err
	}
	pumpSocket(ctx, conn, brw.Reader, write, send, recv)
}

// socketFrame is a control frame the reader passes on, for the one goroutine
// that writes to the connection to answer.
type socketFrame struct {
	opcode  byte
	payload []byte
}

// pumpSocket writes the values of send and reads into recv until the peer leaves,
// the channel closes or ctx ends. Once it returns the connection is closed and
// its reader is gone. The server's context cannot do this by itself, as net/http
// stops watching a connection once it has been hijacked: the handler's context
// only ends when this returns.
func pumpSocket(ctx context.Context, conn io.Closer, read io.Reader, write func(opcode byte, payload []byte) error, send, recv reflect.Value) {
	var (
		control    = make(chan socketFrame)
		stop       = make(chan struct{})
		readerDone = make(chan struct{})
	)
	go func() {
		defer close(readerDone)
		readSocket(read, control, stop, recv)
	}()
	stopClosing := context.AfterFunc(ctx, func() { conn.Close() })
	defer func() {
		stopClosing()
		close(stop)
		conn.Close()
		<-readerDone
	}()
	const (
		ctxDone = iota
		controlFrame
		peerGone
		value
	)
	cases := []reflect.SelectCase{
		ctxDone:      {Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())},
		controlFrame: {Dir: reflect.SelectRecv, Chan: reflect.ValueOf(control)},
		peerGone:     {Dir: reflect.SelectRecv, Chan: reflect.ValueOf(readerDone)},
	}
	if send.IsValid() {
		cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: send})
	}
	for {
		chosen, received, ok := reflect.Select(cases)
		switch chosen {
		case ctxDone, peerGone:
			return
		case controlFrame:
			frame := received.Interface().(socketFrame)
			if frame.opcode == sockClose {
				write(sockClose, frame.payload)
				return
			}
			if err := write(sockPong, frame.payload); err != nil {
				return
			}
		case value:
			if !ok {
				write(sockClose, nil)
				return
			}
			payload, err := json.Marshal(received.Interface())
			if err != nil {
				return
			}
			if err := write(sockText, payload); err != nil {
				return
			}
		}
	}
}

// readSocket runs until the peer leaves or sends what cannot be understood, or
// until stop is closed.
func readSocket(r io.Reader, control chan<- socketFrame, stop <-chan struct{}, recv reflect.Value) {
	for {
		opcode, payload, err := readFrame(r)
		if err != nil {
			return
		}
		switch opcode {
		case sockPing, sockClose:
			select {
			case control <- socketFrame{opcode, payload}:
			case <-stop:
				return
			}
			if opcode == sockClose {
				return
			}
		case sockText:
			if !recv.IsValid() {
				continue
			}
			var value = reflect.New(recv.Type().Elem())
			if err := json.Unmarshal(payload, value.Interface()); err != nil {
				return
			}
			if !sendUnlessStopped(recv, value.Elem(), stop) {
				return
			}
		}
	}
}

func sendUnlessStopped(channel, value reflect.Value, stop <-chan struct{}) bool {
	chosen, _, _ := reflect.Select([]reflect.SelectCase{
		{Dir: reflect.SelectSend, Chan: channel, Send: value},
		{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(stop)},
	})
	return chosen == 0
}

// readFrame reads one frame, with its payload unmasked.
func readFrame(r io.Reader) (opcode byte, payload []byte, err error) {
	var control [2]byte
	if _, err := io.ReadFull(r, control[:]); err != nil {
		return 0, nil, err
	}
	size := uint64(control[1] & frameLength)
	switch size {
	case 126:
		var buf [2]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return 0, nil, err
		}
		size = uint64(binary.BigEndian.Uint16(buf[:]))
	case 127:
		var buf [8]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return 0, nil, err
		}
		size = binary.BigEndian.Uint64(buf[:])
	}
	if size > maxFramePayload {
		return 0, nil, errFrameTooLarge
	}
	var key [4]byte
	if control[1]&frameMasked != 0 {
		if _, err := io.ReadFull(r, key[:]); err != nil {
			return 0, nil, err
		}
	}
	payload = make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	for i := range payload {
		payload[i] ^= key[i%4]
	}
	return control[0] & frameOpcode, payload, nil
}

// appendFrame appends a frame to dst, masked when there is a key, as only a
// client's frames are.
func appendFrame(dst []byte, opcode byte, payload []byte, key []byte) []byte {
	var mask byte
	if key != nil {
		mask = frameMasked
	}
	dst = append(dst, frameFin|opcode)
	switch {
	case len(payload) < 126:
		dst = append(dst, byte(len(payload))|mask)
	case len(payload) <= math.MaxUint16:
		dst = append(dst, 126|mask)
		dst = binary.BigEndian.AppendUint16(dst, uint16(len(payload)))
	default:
		dst = append(dst, 127|mask)
		dst = binary.BigEndian.AppendUint64(dst, uint64(len(payload)))
	}
	if key == nil {
		return append(dst, payload...)
	}
	dst = append(dst, key...)
	for i, b := range payload {
		dst = append(dst, b^key[i%4])
	}
	return dst
}

func sseClientOpen(ctx context.Context, resp *http.Response, recv reflect.Value) {
	if !recv.IsValid() || recv.IsZero() {
		return
	}
	if recv.Kind() != reflect.Chan {
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(nil, maxFramePayload)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := line[6:]
			if data == "" {
				continue
			}

			var value = reflect.New(recv.Type().Elem())
			if err := json.Unmarshal([]byte(data), value.Interface()); err != nil {
				continue
			}

			if !sendUnlessStopped(recv, value.Elem(), ctx.Done()) {
				return
			}
		}
	}
}

// websocketOpen connects to the stream in r, over a websocket or as server-sent
// events if that is what the server answers with. It returns once the connection
// is up, or with the error that kept it from being so. What it returns runs the
// stream to its end and closes recv, which is what the caller of a channel result
// ranges on. send and recv must both be open to the library, so bidirectional.
func websocketOpen(ctx context.Context, client *http.Client, r *http.Request, send, recv reflect.Value, refused func(*http.Response) error) (func(), error) {
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}

	wsReq, err := http.NewRequestWithContext(ctx, "GET", r.URL.String(), nil)
	if err != nil {
		return nil, err
	}

	wsReq.Header.Set("Connection", "Upgrade")
	wsReq.Header.Set("Upgrade", "websocket")
	wsReq.Header.Set("Sec-WebSocket-Version", "13")
	wsReq.Header.Set("Sec-WebSocket-Key", base64.StdEncoding.EncodeToString(key))

	for k, v := range r.Header {
		if k != "Connection" && k != "Upgrade" && k != "Sec-WebSocket-Version" && k != "Sec-WebSocket-Key" {
			wsReq.Header[k] = v
		}
	}

	resp, err := client.Do(wsReq)
	if err != nil {
		return nil, err
	}
	closeRecv := func() {
		if recv.IsValid() {
			recv.Close()
		}
	}
	if resp.StatusCode == 200 && strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		return func() {
			defer resp.Body.Close()
			defer closeRecv()
			sseClientOpen(ctx, resp, recv)
		}, nil
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		defer resp.Body.Close()
		return nil, refused(resp)
	}

	expectedAccept := sha1.Sum([]byte(base64.StdEncoding.EncodeToString(key) + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	conn, ok := resp.Body.(io.ReadWriteCloser)
	switch {
	case !ok:
		resp.Body.Close()
		return nil, fmt.Errorf("the websocket connection to %s cannot be written to", r.URL.Host)
	case !strings.EqualFold(resp.Header.Get("Upgrade"), "websocket"),
		resp.Header.Get("Sec-WebSocket-Accept") != base64.StdEncoding.EncodeToString(expectedAccept[:]):
		conn.Close()
		return nil, fmt.Errorf("%s refused the websocket handshake", r.URL.Host)
	}
	write := func(opcode byte, payload []byte) error {
		var mask [4]byte
		if _, err := rand.Read(mask[:]); err != nil {
			return err
		}
		_, err := conn.Write(appendFrame(nil, opcode, payload, mask[:]))
		return err
	}
	return func() {
		defer closeRecv()
		pumpSocket(ctx, conn, conn, write, send, recv)
	}, nil
}
