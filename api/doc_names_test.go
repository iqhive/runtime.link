package api

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/api/xray"
)

type namedTraceAPI struct {
	Hello func(ctx context.Context, who string) (greeting string, err error)
}

type namedTraceSuite struct {
	TestingFramework
	API namedTraceAPI
}

func (s *namedTraceSuite) TestHello(ctx context.Context) error {
	_, err := s.API.Hello(ctx, "world")
	return err
}

func TestDocumentationArgNames(t *testing.T) {
	useSource(t, srcFile(`
type namedTraceAPI struct {
	Hello func(ctx context.Context, who string) (greeting string, err error)
}
`))
	doc := Documentation(func(context.Context) (Examples, error) {
		return &namedTraceSuite{
			API: namedTraceAPI{
				Hello: func(_ context.Context, who string) (string, error) {
					return "hi " + who, nil
				},
			},
		}, nil
	})
	exec, ok := doc.Test(t.Context(), "TestHello")
	if !ok {
		t.Fatal("TestHello did not run")
	}
	if exec.Error != "" {
		t.Fatalf("execution error: %s", exec.Error)
	}
	var found bool
	for _, event := range exec.Trace {
		if event.Call != "Hello" {
			continue
		}
		found = true
		if !reflect.DeepEqual(event.ArgNames, []string{"who"}) {
			t.Errorf("ArgNames = %q", event.ArgNames)
		}
		if !strings.Contains(string(event.Args), "world") {
			t.Errorf("Args = %s", event.Args)
		}
		if strings.Contains(string(event.Args), "Context") {
			t.Errorf("Args leaked context: %s", event.Args)
		}
	}
	if !found {
		t.Fatalf("no Hello call in trace: %+v", exec.Trace)
	}
}

func TestDocumentationArgNamesJSON(t *testing.T) {
	t.Parallel()
	data, err := json.Marshal(struct {
		ArgNames []string `json:"argNames,omitzero"`
	}{ArgNames: []string{"who"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"argNames"`) {
		t.Errorf("named = %s", data)
	}
}

type timedTraceSuite struct {
	TestingFramework
	API namedTraceAPI
}

func (s *timedTraceSuite) TestHello(ctx context.Context) error {
	s.Guide("says hello")
	_, err := s.API.Hello(ctx, "world")
	return err
}

func TestTraceEventsCarryTheirTime(t *testing.T) {
	doc := Documentation(func(context.Context) (Examples, error) {
		return &timedTraceSuite{
			API: namedTraceAPI{
				Hello: func(context.Context, string) (string, error) { return "hi", nil },
			},
		}, nil
	})
	before := time.Now()
	exec, ok := doc.Test(t.Context(), "TestHello")
	after := time.Now()
	if !ok || exec.Error != "" {
		t.Fatalf("ok=%v err=%s", ok, exec.Error)
	}
	if len(exec.Trace) == 0 {
		t.Fatal("empty trace")
	}
	for _, event := range exec.Trace {
		if event.Time.Before(before) || event.Time.After(after) {
			t.Errorf("event %+v has a time outside of the run", event)
		}
	}
}

// The same, for the calls an implementation makes, which xray records.
func TestInternalTraceEventsCarryTheirTime(t *testing.T) {
	var internal namedTraceAPI
	internal.Hello = func(context.Context, string) (string, error) { return "hi", nil }
	xray.Attach(&internal)
	doc := Documentation(func(context.Context) (Examples, error) {
		return &timedTraceSuite{
			API: namedTraceAPI{Hello: func(ctx context.Context, who string) (string, error) {
				return internal.Hello(ctx, who)
			}},
		}, nil
	})
	before := time.Now()
	exec, ok := doc.Test(t.Context(), "TestHello")
	if !ok || exec.Error != "" {
		t.Fatalf("ok=%v err=%s", ok, exec.Error)
	}
	var calls int
	for _, event := range exec.Trace {
		if event.Call != "" {
			calls++
			if event.Time.Before(before) {
				t.Errorf("event %+v has a time outside of the run", event)
			}
		}
	}
	if calls != 2 {
		t.Errorf("got %d calls in %+v", calls, exec.Trace)
	}
}

type nilServiceAPI struct {
	Specification
	F func()
}

type nilServiceSuite struct {
	TestingFramework
	Svc *nilServiceAPI

	ran bool
}

func (s *nilServiceSuite) Unrelated(context.Context) error {
	s.ran = true
	return nil
}

func (s *nilServiceSuite) UsesService(context.Context) error {
	s.Svc.F()
	return nil
}

func TestNilServiceOnSuite(t *testing.T) {
	suite := &nilServiceSuite{}
	doc := Documentation(func(context.Context) (Examples, error) { return suite, nil })
	example, ok := doc.Example(t.Context(), "Unrelated")
	if !ok || example.Error != nil || !suite.ran {
		t.Errorf("ok=%v err=%v ran=%v", ok, example.Error, suite.ran)
	}
	// Reaching the nil pointer is the example's own failure, and reported as one.
	example, ok = doc.Example(t.Context(), "UsesService")
	if !ok || !example.Panic {
		t.Errorf("ok=%v panic=%v err=%v", ok, example.Panic, example.Error)
	}
}
