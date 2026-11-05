package xray_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/example/petstore"
	"github.com/iqhive/runtime.link/api/rest"
	"github.com/iqhive/runtime.link/api/xray"
)

func TestRay(t *testing.T) {
	var ctx = xray.NewContext(context.Background())
	var Petstore = api.Import[petstore.API](rest.API, "http://localhost:9999", nil)

	Petstore.AddPet(ctx, petstore.Pet{})

	for xray.ContextHas[*http.Request](ctx) {
		req := xray.ContextGet[*http.Request](ctx)
		t.Logf("Request: %s %s", req.Method, req.URL)
		t.Logf("Body: %s", req.Body)
	}
}

func TestContextGetWithoutXray(t *testing.T) {
	if got := xray.ContextGet[int](context.Background()); got != 0 {
		t.Errorf("plain context: got %d", got)
	}
	off := xray.ContextOff(xray.NewContext(context.Background()))
	if got := xray.ContextGet[int](off); got != 0 {
		t.Errorf("ContextOff: got %d", got)
	}
	if got := xray.ContextGet[string](xray.NewContext(context.Background())); got != "" {
		t.Errorf("nothing recorded: got %q", got)
	}
}

// Two readers can both see the one value in a queue. One of them used to wait
// for a second value while holding the read lock, which blocked ContextAdd, the
// only way for that value to arrive, for good.
func TestConcurrentContextGetDoesNotHang(t *testing.T) {
	ctx := xray.NewContext(context.Background())
	const iterations = 20000
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for i := range iterations {
			xray.ContextAdd(ctx, i)
			var readers sync.WaitGroup
			for range 2 {
				readers.Go(func() { xray.ContextGet[int](ctx) })
			}
			readers.Wait()
		}
	}()
	select {
	case <-finished:
	case <-time.After(30 * time.Second):
		t.Fatal("a ContextGet hung while holding the read lock")
	}
}

func TestContextQueueKeepsOrderAcrossGrowth(t *testing.T) {
	ctx := xray.NewContext(context.Background())
	for i := range 10 {
		xray.ContextAdd(ctx, i)
	}
	for want := range 10 {
		if !xray.ContextHas[int](ctx) {
			t.Fatalf("value %d is missing", want)
		}
		if got := xray.ContextGet[int](ctx); got != want {
			t.Errorf("got %d, want %d", got, want)
		}
	}
}

type closeRecorder struct {
	io.Reader
	closed int
}

func (c *closeRecorder) Close() error {
	c.closed++
	return errors.New("closed")
}

func TestNewReaderClosesWrappedReader(t *testing.T) {
	source := &closeRecorder{Reader: strings.NewReader("body")}
	r := xray.NewReader(xray.NewContext(context.Background()), source)
	data, err := io.ReadAll(r)
	if err != nil || string(data) != "body" {
		t.Fatalf("read %q, %v", data, err)
	}
	if r.String() != "body" {
		t.Errorf("String() = %q", r.String())
	}
	if source.closed != 0 {
		t.Error("closed before the caller asked")
	}
	if err := r.Close(); err == nil || err.Error() != "closed" {
		t.Errorf("Close() = %v", err)
	}
	if source.closed != 1 {
		t.Errorf("closed %d times", source.closed)
	}
}

func TestNewReaderClosesReaderThatFailed(t *testing.T) {
	source := &closeRecorder{Reader: iotest.ErrReader(errors.New("broken"))}
	r := xray.NewReader(xray.NewContext(context.Background()), source)
	if _, err := io.ReadAll(r); err == nil || err.Error() != "broken" {
		t.Errorf("read error = %v", err)
	}
	r.Close()
	if source.closed != 1 {
		t.Errorf("closed %d times", source.closed)
	}
}

func TestNewReaderWithoutCloser(t *testing.T) {
	r := xray.NewReader(xray.NewContext(context.Background()), strings.NewReader("body"))
	if err := r.Close(); err != nil {
		t.Errorf("Close() = %v", err)
	}
}
