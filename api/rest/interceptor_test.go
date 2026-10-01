package rest_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"runtime.link/api"
	"runtime.link/api/rest"
)

type interceptedAPI struct {
	api.Specification

	Echo func(context.Context, string) (string, error) `rest:"POST /echo/{name=%v}"`
}

func interceptedClient(t *testing.T, sent *int) *interceptedAPI {
	t.Helper()
	handler, err := rest.Handler(nil, interceptedAPI{
		Echo: func(ctx context.Context, name string) (string, error) {
			*sent++
			return "hello " + name, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := api.Import[interceptedAPI](rest.API, server.URL, server.Client())
	return &client
}

func respond(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestInterceptorChangesRequest(t *testing.T) {
	var sent int
	client := interceptedClient(t, &sent)
	var seen, status string
	ctx := rest.Interceptor(context.Background(), func(req *http.Request, send func(*http.Request) (*http.Response, error)) (*http.Response, error) {
		seen = req.Method + " " + req.URL.Path
		req = req.Clone(req.Context())
		req.URL.Path = "/echo/bob"
		resp, err := send(req)
		if resp != nil {
			status = resp.Status
		}
		return resp, err
	})
	got, err := client.Echo(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello bob" || seen != "POST /echo/alice" || sent != 1 || !strings.HasPrefix(status, "200") {
		t.Fatalf("got %q, saw %q, sent %d, status %q", got, seen, sent, status)
	}
}

func TestInterceptorMakesUpResponse(t *testing.T) {
	var sent int
	client := interceptedClient(t, &sent)
	ctx := rest.Interceptor(context.Background(), func(req *http.Request, send func(*http.Request) (*http.Response, error)) (*http.Response, error) {
		return respond(http.StatusOK, `"made up"`), nil
	})
	got, err := client.Echo(ctx, "alice")
	if err != nil || got != "made up" || sent != 0 {
		t.Fatalf("got %q, %v after sending %d", got, err, sent)
	}

	ctx = rest.Interceptor(context.Background(), func(req *http.Request, send func(*http.Request) (*http.Response, error)) (*http.Response, error) {
		return respond(http.StatusConflict, `{"message":"no"}`), nil
	})
	if _, err := client.Echo(ctx, "alice"); err == nil || sent != 0 {
		t.Fatalf("made up failure: %v after sending %d", err, sent)
	}

	ctx = rest.Interceptor(context.Background(), func(req *http.Request, send func(*http.Request) (*http.Response, error)) (*http.Response, error) {
		return nil, nil
	})
	if _, err := client.Echo(ctx, "alice"); err == nil {
		t.Fatal("no response was not an error")
	}
}

func TestInterceptorsNest(t *testing.T) {
	var sent int
	client := interceptedClient(t, &sent)
	var order []string
	trace := func(name string) func(*http.Request, func(*http.Request) (*http.Response, error)) (*http.Response, error) {
		return func(req *http.Request, send func(*http.Request) (*http.Response, error)) (*http.Response, error) {
			order = append(order, name+" request")
			resp, err := send(req)
			order = append(order, name+" response")
			return resp, err
		}
	}
	ctx := rest.Interceptor(context.Background(), trace("outer"))
	ctx = rest.Interceptor(ctx, trace("inner"))
	if _, err := client.Echo(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(order, ", "); got != "outer request, inner request, inner response, outer response" {
		t.Fatalf("order: %s", got)
	}

	errBlocked := errors.New("blocked")
	ctx = rest.Interceptor(ctx, func(*http.Request, func(*http.Request) (*http.Response, error)) (*http.Response, error) {
		return nil, errBlocked
	})
	if _, err := client.Echo(ctx, "alice"); !errors.Is(err, errBlocked) {
		t.Fatalf("got %v, want the interceptor's error", err)
	}
}
