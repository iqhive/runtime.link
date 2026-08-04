package rest

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"runtime.link/api"
)

// sampleAPI is a minimal rest-tagged API whose call is captured in the trace.
type sampleAPI struct {
	api.Specification

	Echo func(context.Context, string) (string, error) `rest:"POST /echo (message) message"`
}

// sampleSuite is a test suite that exercises sampleAPI so the recorded trace
// should carry a sampled downstream request/response.
type sampleSuite struct {
	api.TestingFramework

	Sample sampleAPI
}

// TestEcho calls the rest-tagged Echo endpoint.
func (s *sampleSuite) TestEcho(ctx context.Context) error {
	_, err := s.Sample.Echo(ctx, "hello")
	return err
}

func newSampleSuite(ctx context.Context) (api.Examples, error) {
	return &sampleSuite{
		Sample: sampleAPI{
			Echo: func(ctx context.Context, message string) (string, error) { return message, nil },
		},
	}, nil
}

// TestTraceSamplesDownstreamHTTP verifies that the sampler registered by this
// package's init attaches the downstream HTTP url/request/response to the
// recorded trace events for calls that carry a rest tag.
func TestTraceSamplesDownstreamHTTP(t *testing.T) {
	var doc api.Documentation = newSampleSuite
	exec, ok := doc.Test(t.Context(), "TestEcho")
	if !ok {
		t.Fatal("TestEcho did not run")
	}
	var found bool
	for _, event := range exec.Trace {
		if event.Call != "Echo" {
			continue
		}
		found = true
		if event.URL == "" {
			t.Error("Echo event has no sampled URL")
		}
		if !strings.HasPrefix(event.URL, "POST ") || !strings.Contains(event.URL, "/echo") {
			t.Errorf("Echo URL = %q, want POST .../echo", event.URL)
		}
		if len(event.Resp) == 0 {
			t.Error("Echo event has no sampled response")
		}
		if !strings.Contains(string(event.Resp), "hello") {
			t.Errorf("Echo response = %q, want it to contain the echoed message", event.Resp)
		}
		// The sampled request body must carry the message, not the leading
		// context.Context argument (which would serialize as {"Context":...}).
		if !strings.Contains(string(event.Req), "hello") {
			t.Errorf("Echo request = %q, want it to contain the message body", event.Req)
		}
		if strings.Contains(string(event.Req), "Context") {
			t.Errorf("Echo request = %q, leaked the context.Context argument", event.Req)
		}
	}
	if !found {
		t.Fatalf("no Echo call captured in trace: %+v", exec.Trace)
	}
}

// TestSampleSkipsLeadingContext exercises the sampler directly with an argument
// list that retains the leading context.Context (as xray-recorded calls do),
// verifying the context is not serialized into the request body.
func TestSampleSkipsLeadingContext(t *testing.T) {
	field, ok := reflect.TypeOf(sampleAPI{}).FieldByName("Echo")
	if !ok {
		t.Fatal("sampleAPI has no Echo field")
	}
	impl := reflect.ValueOf(func(ctx context.Context, message string) (string, error) { return message, nil })
	fn := api.Function{
		Name: field.Name,
		Tags: field.Tag,
		Type: field.Type,
		Impl: impl,
	}
	// args as xray records them: the full list including the leading context,
	// each carrying the function's declared parameter type (so the context is
	// the interface type context.Context, not its concrete implementation).
	ctxArg := reflect.New(field.Type.In(0)).Elem()
	ctxArg.Set(reflect.ValueOf(context.Background()))
	args := []reflect.Value{ctxArg, reflect.ValueOf("hello")}
	vals := []reflect.Value{reflect.ValueOf("hello")}
	url, req, resp, err := sample(fn, args, vals)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "POST ") || !strings.Contains(url, "/echo") {
		t.Errorf("url = %q, want POST .../echo", url)
	}
	if !strings.Contains(string(req), "hello") {
		t.Errorf("req = %q, want it to contain the message body", req)
	}
	if strings.Contains(string(req), "Context") {
		t.Errorf("req = %q, leaked the context.Context argument", req)
	}
	if !strings.Contains(string(resp), "hello") {
		t.Errorf("resp = %q, want the echoed message", resp)
	}
}
