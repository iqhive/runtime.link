package rest

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"reflect"
	"strings"
)

// websocketServeHTTP currently only supports websockets for the case of returning a receive-only channel
// from an API endpoint. In future, it may be extended to support incoming messages as well.
func websocketServeHTTP(ctx context.Context, r *http.Request, rw http.ResponseWriter, channel reflect.Value) {
	const (
		sockContinue = 0x0
		sockText     = 0x1
		sockBinary   = 0x2
		sockClose    = 0x8
		sockPing     = 0x9
		sockPong     = 0xA

		fin  = 0b10000000
		mask = 0b10000000
	)
	if r.Method != "GET" {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !strings.Contains(r.Header.Get("Connection"), "Upgrade") || r.Header.Get("Upgrade") != "websocket" {
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
	var w io.Writer = rw
	var body io.Reader = r.Body
	hijacker, ok := w.(http.Hijacker)
	if ok {
		conn, brw, err := hijacker.Hijack()
		brw.Flush()
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		defer conn.Close()
		w = conn
		body = conn
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	var pongs = make(chan struct{})
	cases := []reflect.SelectCase{
		{Dir: reflect.SelectRecv, Chan: channel},
		{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())},
		{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(pongs)},
	}
	go func() { // handle reads.
		const (
			opcode = 0b00001111
			length = 0b01111111
			masked = 0b10000000
		)
		for {
			var control [2]byte
			if _, err := io.ReadAtLeast(body, control[:], 2); err != nil {
				return
			}
			size := uint(control[1] & length)
			switch size {
			case 126:
				var buf [2]byte
				if _, err := io.ReadAtLeast(body, buf[:], 2); err != nil {
					return
				}
				size = uint(binary.BigEndian.Uint16(buf[:]))
			case 127:
				var buf [8]byte
				if _, err := io.ReadAtLeast(body, buf[:], 8); err != nil {
					return
				}
				size = uint(binary.BigEndian.Uint64(buf[:]))
			}
			var key uint32
			if control[1]&masked != 0 {
				var buf [4]byte
				if _, err := io.ReadAtLeast(body, buf[:], 4); err != nil {
					return
				}
				key = binary.BigEndian.Uint32(buf[:])
			}
			if size > 0 {
				io.CopyN(io.Discard, body, int64(size))
			}
			_ = key
			if control[0]&opcode == sockPing {
				select {
				case pongs <- struct{}{}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	var frame [16]byte
	for {
		closing := false
		chosen, value, ok := reflect.Select(cases)
		if chosen == 1 {
			return // context cancelled
		}
		if !ok {
			closing = true
		}
		var b []byte
		var err error
		if !closing {
			if chosen == 0 {
				b, err = json.Marshal(value.Interface())
				if err != nil {
					return
				}
			}
		}
		frame := frame[:0]
		if closing {
			frame = append(frame, fin|sockClose)
		} else if chosen == 2 {
			frame = append(frame, fin|sockPong)
		} else {
			frame = append(frame, fin|sockText)
		}
		switch {
		case len(b) < 126:
			frame = append(frame, byte(len(b)))
		case len(b) < math.MaxUint16:
			frame = append(frame, 126)
			binary.BigEndian.AppendUint16(frame, uint16(len(b)))
		default:
			frame = append(frame, 127)
			binary.BigEndian.AppendUint64(frame, uint64(len(b)))
		}
		if _, err := w.Write(frame); err != nil {
			return
		}
		if _, err := w.Write(b); err != nil {
			return
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		if closing {
			break
		}
	}
}
