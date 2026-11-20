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
					var resp = httptest.NewRecorder()
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
					args[0].Invoke(jsResponse(resp))
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

// jsResponse returns the Response of fetch for what a handler wrote: its status
// and headers, and its body. Response throws for a status that has no body when
// it is given one, an empty one too, and for a header or a status it does not
// take, and a throw is a panic here, in a goroutine where it would end the
// process. Whatever it is, it is answered as the server error it is.
func jsResponse(resp *httptest.ResponseRecorder) (response js.Value) {
	defer func() {
		if failure := recover(); failure != nil {
			log.Printf("rest: cannot answer with status %d: %v", resp.Code, failure)
			response = js.Global().Get("Response").New(js.Null(), map[string]any{
				"status": http.StatusInternalServerError,
			})
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
	var body = js.Null()
	if statusHasBody(resp.Code) {
		body = js.Global().Get("Uint8Array").New(resp.Body.Len())
		js.CopyBytesToJS(body, resp.Body.Bytes())
	}
	return js.Global().Get("Response").New(body, map[string]any{
		"status":  resp.Code,
		"headers": headers,
	})
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
	var result = make(chan js.Value)
	var issues = make(chan error)
	promise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
		result <- args[0]
		return nil
	})).Call("catch", js.FuncOf(func(this js.Value, args []js.Value) any {
		issues <- errors.New(args[0].String())
		return nil
	}))
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
