package rest

import (
	"context"
	_ "embed"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"syscall/js"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/xray"
)

// ListenAndServe starts a HTTP server that serves supported API
// types. If the [Authenticator] is nil, requests will not require
// any authentication.
//
// On Deno, the address a request came from is taken from
// X-Forwarded-For. Deno Deploy sets that header itself; anywhere
// else, whatever sits in front must strip one a client sends, or
// the client names the address its requests appear to come from.
func ListenAndServe(addr string, auth api.Auth[*http.Request], impl any) error {
	handler, err := Handler(auth, impl)
	if err != nil {
		return xray.New(err)
	}
	Deno := js.Global().Get("Deno")
	if !Deno.IsUndefined() {
		jsHandler := js.FuncOf(func(this js.Value, args []js.Value) any {
			var fetchReq = args[0]
			var promiseHandler js.Func
			promiseHandler = js.FuncOf(func(this js.Value, args []js.Value) any {
				go func() {
					// Every function made for a request is released once it has
					// been answered, as is its context: none of them is collected.
					defer promiseHandler.Release()
					var resp = &jsResponseWriter{ResponseRecorder: httptest.NewRecorder(), answer: args[0]}
					var body io.ReadCloser
					if !fetchReq.Get("body").IsNull() {
						body = &jsReadableStream{body: fetchReq.Get("body").Call("getReader")}
					}
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					abort := js.FuncOf(func(this js.Value, args []js.Value) any {
						cancel()
						return nil
					})
					defer abort.Release()
					fetchReq.Get("signal").Call("addEventListener", "abort", abort)
					defer fetchReq.Get("signal").Call("removeEventListener", "abort", abort)
					var req = httptest.NewRequestWithContext(ctx, fetchReq.Get("method").String(), fetchReq.Get("url").String(), body)
					req.Header = make(http.Header)
					header := js.FuncOf(func(this js.Value, args []js.Value) any {
						var key = args[1].String()
						var value = args[0].String()
						req.Header.Set(key, value)
						return nil
					})
					fetchReq.Get("headers").Call("forEach", header)
					header.Release()
					// Read with the method of Headers, which has no properties
					// by the names of its headers, and gives null for one that
					// was not sent.
					if forwarded := fetchReq.Get("headers").Call("get", "x-forwarded-for"); forwarded.Type() == js.TypeString {
						req.RemoteAddr = forwarded.String()
					}
					serveRecovering(handler, resp, req)
					resp.finish()
				}()
				return nil
			})
			promise := js.Global().Get("Promise").New(promiseHandler)
			return promise
		})
		exit := make(chan struct{})
		server := Deno.Call("serve", jsHandler)
		server.Get("finished").Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
			close(exit)
			return nil
		}))
		<-exit
		return nil
	}
	return http.ListenAndServe(addr, handler)
}

// jsResponseWriter keeps a response in memory and answers with it once its
// handler has returned, unless the handler flushes it. A stream of events is
// flushed after each of them, and may never end: from its first flush on, it is
// answered with a body that is sent as it is written. Kept until the handler
// returned, its events would reach nobody, and be held without bound.
type jsResponseWriter struct {
	*httptest.ResponseRecorder

	// answer resolves the promise of the request with its response, once.
	answer js.Value

	// stream is the controller of the body of a response that was flushed.
	stream js.Value

	isStreamed bool

	// isUnread is set once the stream cannot be written to, as its reader
	// has gone, or as the response it was to be the body of was refused.
	isUnread bool
}

// Flush sends what was written so far, which is what a handler asks for when
// it flushes.
func (w *jsResponseWriter) Flush() {
	w.ResponseRecorder.Flush()
	if !w.isStreamed {
		if !statusHasBody(w.Code) {
			return
		}
		w.isStreamed = true
		start := js.FuncOf(func(this js.Value, args []js.Value) any {
			w.stream = args[0]
			return nil
		})
		// Called while the stream is made, and not after.
		defer start.Release()
		body := js.Global().Get("ReadableStream").New(map[string]any{"start": start})
		response, ok := jsResponseOf(w.ResponseRecorder, body)
		w.isUnread = !ok
		w.answer.Invoke(response)
	}
	w.sendWritten()
}

// sendWritten moves what the handler wrote since the last flush to the stream.
func (w *jsResponseWriter) sendWritten() {
	if w.Body.Len() == 0 {
		return
	}
	chunk := js.Global().Get("Uint8Array").New(w.Body.Len())
	js.CopyBytesToJS(chunk, w.Body.Bytes())
	w.Body.Reset()
	w.callStream("enqueue", chunk)
}

// callStream calls a method of the stream's controller. It throws once the
// stream was cancelled, which is what happens when the client goes away, and a
// throw is a panic here: what is written after that is dropped.
func (w *jsResponseWriter) callStream(method string, args ...any) {
	if w.isUnread {
		return
	}
	defer func() {
		if recover() != nil {
			w.isUnread = true
		}
	}()
	w.stream.Call(method, args...)
}

// finish answers the request once its handler has returned, or ends the body
// of the answer it was given when it was flushed.
func (w *jsResponseWriter) finish() {
	if !w.isStreamed {
		w.answer.Invoke(jsResponse(w.ResponseRecorder))
		return
	}
	w.sendWritten()
	w.callStream("close")
}

// jsResponse returns the Response of fetch for what a handler wrote: its status
// and headers, and its body. Response throws for a status that has no body when
// it is given one, an empty one too, and for a header or a status it does not
// take, and a throw is a panic here, in a goroutine where it would end the
// process. Whatever it is, it is answered as the server error it is.
func jsResponse(resp *httptest.ResponseRecorder) js.Value {
	var body = js.Null()
	if statusHasBody(resp.Code) {
		body = js.Global().Get("Uint8Array").New(resp.Body.Len())
		js.CopyBytesToJS(body, resp.Body.Bytes())
	}
	response, _ := jsResponseOf(resp, body)
	return response
}

// jsResponseOf is [jsResponse] with the body given. ok is false when the
// response is the server error that stands in for one that was refused.
func jsResponseOf(resp *httptest.ResponseRecorder, body js.Value) (response js.Value, ok bool) {
	defer func() {
		if failure := recover(); failure != nil {
			log.Printf("rest: cannot answer with status %d: %v", resp.Code, failure)
			response = js.Global().Get("Response").New(js.Null(), map[string]any{
				"status": http.StatusInternalServerError,
			})
			ok = false
		}
	}()
	headers := js.Global().Get("Headers").New()
	// The headers as they were when the status was written, as those set
	// after it are not sent.
	for name, values := range resp.Result().Header {
		for _, value := range values {
			headers.Call("append", name, value)
		}
	}
	return js.Global().Get("Response").New(body, map[string]any{
		"status":  resp.Code,
		"headers": headers,
	}), true
}

// statusHasBody reports whether a response of this status may have a body,
// which those that fetch calls the null body statuses may not.
func statusHasBody(status int) bool {
	switch status {
	case http.StatusSwitchingProtocols, http.StatusNoContent, http.StatusResetContent, http.StatusNotModified:
		return false
	}
	return true
}

func await2(promise js.Value) (a, b js.Value, err error) {
	if promise.IsUndefined() {
		return js.Undefined(), js.Undefined(), errors.New("undefined promise")
	}
	// Room for the one value either gets, so that neither function waits for
	// this one to read it while it is being released.
	var result = make(chan js.Value, 1)
	var issues = make(chan error, 1)
	then := js.FuncOf(func(this js.Value, args []js.Value) any {
		result <- args[0]
		return nil
	})
	catch := js.FuncOf(func(this js.Value, args []js.Value) any {
		issues <- errors.New(args[0].String())
		return nil
	})
	// A promise settles once, so only one of the two is ever called, and
	// neither after that: a function that is not released is never collected,
	// and this is called for every chunk of a body.
	defer then.Release()
	defer catch.Release()
	promise.Call("then", then).Call("catch", catch)
	select {
	case value := <-result:
		return value.Get("done"), value.Get("value"), nil
	case err := <-issues:
		return js.Undefined(), js.Undefined(), err
	}
}

type jsReadableStream struct {
	body js.Value
	data []byte
}

func (stream *jsReadableStream) Read(p []byte) (n int, err error) {
	if len(stream.data) > 0 {
		n = copy(p, stream.data)
		stream.data = stream.data[n:]
		return n, nil
	}
	if stream.body.IsUndefined() {
		return 0, io.EOF
	}
	done, chunk, err := await2(stream.body.Call("read"))
	if err != nil {
		return 0, err
	}
	if done.Bool() {
		return 0, io.EOF
	}
	n = js.CopyBytesToGo(p, chunk)
	if n < chunk.Length() {
		stream.data = stream.data[:0]
		if len(stream.data) < chunk.Length() {
			stream.data = make([]byte, chunk.Length())
		}
		stream.data = stream.data[0:js.CopyBytesToGo(stream.data, chunk.Call("slice", n, chunk.Length()))]
	}
	return n, nil
}

func (stream *jsReadableStream) Close() error {
	stream.body.Call("cancel")
	return nil
}
