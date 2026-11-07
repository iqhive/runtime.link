package rest_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
)

func TestQueryExplicitZerosRoundTrip(t *testing.T) {
	type filter struct {
		Limit   *int     `json:"limit"`
		Name    *string  `json:"name"`
		Numbers []int    `json:"n"`
		Flags   []bool   `json:"b"`
		Words   []string `json:"s"`
	}
	type API struct {
		api.Specification
		Numbers  func(context.Context, []int) ([]int, error)                   `rest:"GET /numbers?v=%v"`
		Flags    func(context.Context, []bool) ([]bool, error)                 `rest:"GET /flags?v=%v"`
		Words    func(context.Context, []string) ([]string, error)             `rest:"GET /words?v=%v"`
		Int      func(context.Context, *int) (string, error)                   `rest:"GET /int?v=%v"`
		Bool     func(context.Context, *bool) (string, error)                  `rest:"GET /bool?v=%v"`
		String   func(context.Context, *string) (string, error)                `rest:"GET /string?v=%v"`
		Map      func(context.Context, map[string]int) (map[string]int, error) `rest:"GET /map?%v"`
		Expanded func(context.Context, filter) (filter, error)                 `rest:"GET /expanded?%v"`
	}
	h, err := rest.Handler(nil, API{
		Numbers: func(_ context.Context, v []int) ([]int, error) { return v, nil },
		Flags:   func(_ context.Context, v []bool) ([]bool, error) { return v, nil },
		Words:   func(_ context.Context, v []string) ([]string, error) { return v, nil },
		Int: func(_ context.Context, v *int) (string, error) {
			if v == nil {
				return "absent", nil
			}
			return fmt.Sprint(*v), nil
		},
		Bool: func(_ context.Context, v *bool) (string, error) {
			if v == nil {
				return "absent", nil
			}
			return fmt.Sprint(*v), nil
		},
		String: func(_ context.Context, v *string) (string, error) {
			if v == nil {
				return "absent", nil
			}
			return "present:" + *v, nil
		},
		Map:      func(_ context.Context, v map[string]int) (map[string]int, error) { return v, nil },
		Expanded: func(_ context.Context, v filter) (filter, error) { return v, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	ctx := context.Background()
	for _, want := range [][]int{{0, 1, 0}, {0, 0}, {2, 3}} {
		if got, err := client.Numbers(ctx, want); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("numbers: %v, %v; want %v", got, err, want)
		}
	}
	{
		want := []bool{false, true, false}
		if got, err := client.Flags(ctx, want); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("flags: %v, %v; want %v", got, err, want)
		}
	}
	{
		want := []string{"", "x", ""}
		if got, err := client.Words(ctx, want); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("words: %q, %v; want %q", got, err, want)
		}
	}
	zero, no, empty := 0, false, ""
	for _, tc := range []struct {
		name, want string
		call       func() (string, error)
	}{
		{"zero", "0", func() (string, error) { return client.Int(ctx, &zero) }},
		{"false", "false", func() (string, error) { return client.Bool(ctx, &no) }},
		{"empty", "present:", func() (string, error) { return client.String(ctx, &empty) }},
		{"nil int", "absent", func() (string, error) { return client.Int(ctx, nil) }},
		{"nil bool", "absent", func() (string, error) { return client.Bool(ctx, nil) }},
		{"nil string", "absent", func() (string, error) { return client.String(ctx, nil) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := tc.call(); err != nil || got != tc.want {
				t.Fatalf("%q, %v; want %q", got, err, tc.want)
			}
		})
	}
	wantMap := map[string]int{"zero": 0, "one": 1}
	if got, err := client.Map(ctx, wantMap); err != nil || !reflect.DeepEqual(got, wantMap) {
		t.Errorf("map: %v, %v; want %v", got, err, wantMap)
	}
	want := filter{Limit: &zero, Name: &empty, Numbers: []int{0, 1, 0}, Flags: []bool{false, true}, Words: []string{"", "x"}}
	if got, err := client.Expanded(ctx, want); err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("expanded: %+v, %v; want %+v", got, err, want)
	}
}
