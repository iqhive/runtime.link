package rest

import (
	"context"
	_ "embed"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"syscall/js"

	"runtime.link/api"
	"runtime.link/api/xray"
)

// ListenAndServe starts a HTTP server that serves supported API
// types. If the [Authenticator] is nil, requests will not require
// any authentication.
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
					var resp = httptest.NewRecorder()
					var body io.ReadCloser
					if !fetchReq.Get("body").IsNull() {
						body = &jsReadableStream{body: fetchReq.Get("body").Call("getReader")}
					}
					ctx, cancel := context.WithCancel(context.Background())
					fetchReq.Get("signal").Call("addEventListener", "abort", js.FuncOf(func(this js.Value, args []js.Value) any {
						cancel()
						return nil
					}))
					var req = httptest.NewRequestWithContext(ctx, fetchReq.Get("method").String(), fetchReq.Get("url").String(), body)
					req.Header = make(http.Header)
					fetchReq.Get("headers").Call("forEach", js.FuncOf(func(this js.Value, args []js.Value) any {
						var key = args[1].String()
						var value = args[0].String()
						req.Header.Set(key, value)
						return nil
					}))
					req.RemoteAddr = fetchReq.Get("headers").Get("x-forwarded-for").String()
					handler.ServeHTTP(resp, req)
					var data = js.Global().Get("Uint8Array").New(resp.Body.Len())
					js.CopyBytesToJS(data, resp.Body.Bytes())
					args[0].Invoke(js.Global().Get("Response").New(data, map[string]any{
						"status": resp.Code,
					}))
					promiseHandler.Release()
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
