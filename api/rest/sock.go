package rest

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"reflect"
	"strings"
)

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
		{Dir: reflect.SelectRecv, Chan: send},
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
			switch control[0] & opcode {
			case sockPing:
				if size > 0 {
					io.CopyN(io.Discard, body, int64(size))
				}
				select {
				case pongs <- struct{}{}:
				case <-ctx.Done():
					return
				}
			case sockText:
				var buf = make([]byte, size)
				if _, err := io.ReadAtLeast(body, buf, int(size)); err != nil {
					return
				}
				for i := range buf {
					j := i % 4
					buf[i] = buf[i] ^ byte(key>>(8*j))
				}
				var value = reflect.New(recv.Type().Elem())
				if err := json.Unmarshal(buf, value.Interface()); err != nil {
					return
				}
				recv.Send(value.Elem())
			default:
				if size > 0 {
					io.CopyN(io.Discard, body, int64(size))
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

func sseServeHTTP(ctx context.Context, r *http.Request, rw http.ResponseWriter, send reflect.Value) {
	if r.Method != "GET" {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Connection", "keep-alive")
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	
	rw.WriteHeader(http.StatusOK)
	if flusher, ok := rw.(http.Flusher); ok {
		flusher.Flush()
	}
	
	cases := []reflect.SelectCase{
		{Dir: reflect.SelectRecv, Chan: send},
		{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())},
	}
	
	for {
		chosen, value, ok := reflect.Select(cases)
		if chosen == 1 {
			return // context cancelled
		}
		if !ok {
			break
		}
		
		data, err := json.Marshal(value.Interface())
		if err != nil {
			return
		}
		
		if _, err := fmt.Fprintf(rw, "data: %s\n\n", data); err != nil {
			return
		}
		
		if flusher, ok := rw.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}

func sseClientOpen(ctx context.Context, resp *http.Response, recv reflect.Value) {
	if !recv.IsValid() || recv.IsZero() {
		return
	}
	if recv.Kind() != reflect.Chan || recv.Type().ChanDir() != reflect.RecvDir {
		return
	}
	
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		
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
			
			select {
			case <-ctx.Done():
				return
			default:
				recv.Send(value.Elem())
			}
		}
	}
}

func websocketOpen(ctx context.Context, client *http.Client, r *http.Request, send, recv reflect.Value) {
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

	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		return
	}
	
	wsURL := r.URL.String()
	if strings.HasPrefix(wsURL, "http://") {
		wsURL = "ws://" + wsURL[7:]
	} else if strings.HasPrefix(wsURL, "https://") {
		wsURL = "wss://" + wsURL[8:]
	}
	
	wsReq, err := http.NewRequestWithContext(ctx, "GET", wsURL, nil)
	if err != nil {
		return
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
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == 200 && resp.Header.Get("Content-Type") == "text/event-stream" {
		sseClientOpen(ctx, resp, recv)
		return
	}
	
	if resp.StatusCode != 101 {
		return
	}
	if resp.Header.Get("Upgrade") != "websocket" {
		return
	}
	
	expectedAccept := sha1.Sum([]byte(base64.StdEncoding.EncodeToString(key) + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	if resp.Header.Get("Sec-WebSocket-Accept") != base64.StdEncoding.EncodeToString(expectedAccept[:]) {
		return
	}
	
	hijacker, ok := resp.Body.(interface {
		Hijack() (net.Conn, *bufio.ReadWriter, error)
	})
	if !ok {
		return
	}
	
	conn, brw, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer conn.Close()
	
	if brw != nil {
		brw.Flush()
	}
	
	var pongs = make(chan struct{})
	var cases []reflect.SelectCase
	
	if send.IsValid() && !send.IsZero() {
		cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: send})
	}
	
	cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())})
	
	cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(pongs)})
	
	go func() {
		const (
			opcode = 0b00001111
			length = 0b01111111
			masked = 0b10000000
		)
		
		for {
			var control [2]byte
			if _, err := io.ReadAtLeast(conn, control[:], 2); err != nil {
				return
			}
			
			size := uint(control[1] & length)
			switch size {
			case 126:
				var buf [2]byte
				if _, err := io.ReadAtLeast(conn, buf[:], 2); err != nil {
					return
				}
				size = uint(binary.BigEndian.Uint16(buf[:]))
			case 127:
				var buf [8]byte
				if _, err := io.ReadAtLeast(conn, buf[:], 8); err != nil {
					return
				}
				size = uint(binary.BigEndian.Uint64(buf[:]))
			}
			
			var key uint32
			if control[1]&masked != 0 {
				var buf [4]byte
				if _, err := io.ReadAtLeast(conn, buf[:], 4); err != nil {
					return
				}
				key = binary.BigEndian.Uint32(buf[:])
			}
			
			switch control[0] & opcode {
			case sockPing:
				if size > 0 {
					io.CopyN(io.Discard, conn, int64(size))
				}
				select {
				case pongs <- struct{}{}:
				case <-ctx.Done():
					return
				}
			case sockText:
				if recv.IsValid() && !recv.IsZero() {
					var buf = make([]byte, size)
					if _, err := io.ReadAtLeast(conn, buf, int(size)); err != nil {
						return
					}
					
					for i := range buf {
						j := i % 4
						buf[i] = buf[i] ^ byte(key>>(8*j))
					}
					
					var value = reflect.New(recv.Type().Elem())
					if err := json.Unmarshal(buf, value.Interface()); err != nil {
						return
					}
					recv.Send(value.Elem())
				} else {
					if size > 0 {
						io.CopyN(io.Discard, conn, int64(size))
					}
				}
			default:
				if size > 0 {
					io.CopyN(io.Discard, conn, int64(size))
				}
			}
		}
	}()
	
	var frame [16]byte
	for {
		closing := false
		chosen, value, ok := reflect.Select(cases)
		
		if send.IsValid() && !send.IsZero() {
			if chosen == 1 { // context done
				return
			}
		} else {
			if chosen == 0 { // context done
				return
			}
		}
		
		if !ok {
			closing = true
		}
		
		var b []byte
		var err error
		if !closing {
			if chosen == 0 && send.IsValid() && !send.IsZero() { // send channel
				b, err = json.Marshal(value.Interface())
				if err != nil {
					return
				}
			}
		}
		
		frame := frame[:0]
		if closing {
			frame = append(frame, fin|sockClose)
		} else if chosen == len(cases)-1 { // pongs
			frame = append(frame, fin|sockPong)
		} else if chosen == 0 && send.IsValid() && !send.IsZero() {
			frame = append(frame, fin|sockText)
		} else {
			continue // Skip other cases
		}
		
		switch {
		case len(b) < 126:
			frame = append(frame, byte(len(b))|mask) // Client must mask
		case len(b) < math.MaxUint16:
			frame = append(frame, 126|mask)
			binary.BigEndian.AppendUint16(frame, uint16(len(b)))
		default:
			frame = append(frame, 127|mask)
			binary.BigEndian.AppendUint64(frame, uint64(len(b)))
		}
		
		var maskKey [4]byte
		if _, err := rand.Read(maskKey[:]); err != nil {
			return
		}
		frame = append(frame, maskKey[:]...)
		
		if _, err := conn.Write(frame); err != nil {
			return
		}
		
		if len(b) > 0 {
			masked := make([]byte, len(b))
			for i := range b {
				masked[i] = b[i] ^ maskKey[i%4]
			}
			if _, err := conn.Write(masked); err != nil {
				return
			}
		}
	
		if closing {
			break
		}
	}
}
