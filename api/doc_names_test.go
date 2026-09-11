package api

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
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
