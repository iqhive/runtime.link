package rest_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
)

const secret = "s3cret"

type hiddenProfile struct {
	Title  string `json:"title"`
	Secret string `json:"-"`
	shadow string
}

type hiddenAccount struct {
	Name     string `json:"name"`
	Password string `json:"-"`
	shadow   string

	Nested  hiddenProfile    `json:"nested"`
	Pointer *hiddenProfile   `json:"pointer"`
	Slice   []hiddenProfile  `json:"slice"`
	Array   [1]hiddenProfile `json:"array"`
	Any     any              `json:"any"`
}

func newHiddenProfile() hiddenProfile {
	return hiddenProfile{Title: "t", Secret: secret, shadow: secret}
}

func newHiddenAccount() hiddenAccount {
	profile := newHiddenProfile()
	return hiddenAccount{
		Name:     "alice",
		Password: secret,
		shadow:   secret,
		Nested:   profile,
		Pointer:  &profile,
		Slice:    []hiddenProfile{profile, profile},
		Array:    [1]hiddenProfile{profile},
		Any:      profile,
	}
}

func TestHiddenFieldsNeverLeaveInAResponse(t *testing.T) {
	type API struct {
		api.Specification
		Value   func(context.Context) (hiddenAccount, error)                  `rest:"GET /value"`
		Pointer func(context.Context) (*hiddenAccount, error)                 `rest:"GET /pointer"`
		Slice   func(context.Context) ([]hiddenAccount, error)                `rest:"GET /slice"`
		Map     func(context.Context) (map[string]hiddenProfile, error)       `rest:"GET /map"`
		Named   func(context.Context) (hiddenProfile, []hiddenProfile, error) `rest:"GET /named one,two"`
	}
	h, err := rest.Handler(nil, API{
		Value:   func(context.Context) (hiddenAccount, error) { return newHiddenAccount(), nil },
		Pointer: func(context.Context) (*hiddenAccount, error) { a := newHiddenAccount(); return &a, nil },
		Slice: func(context.Context) ([]hiddenAccount, error) {
			return []hiddenAccount{newHiddenAccount()}, nil
		},
		Map: func(context.Context) (map[string]hiddenProfile, error) {
			return map[string]hiddenProfile{"a": newHiddenProfile()}, nil
		},
		Named: func(context.Context) (hiddenProfile, []hiddenProfile, error) {
			return newHiddenProfile(), []hiddenProfile{newHiddenProfile()}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/value", "/pointer", "/slice", "/map", "/named"} {
		for _, accept := range []string{"", "*/*", "application/json", "text/plain", "application/xml", "multipart/form-data", "application/octet-stream", "application/json+schema"} {
			headers := []string{}
			if accept != "" {
				headers = append(headers, "Accept", accept)
			}
			rec := serve(t, h, "GET", target, "", headers...)
			if strings.Contains(rec.Body.String(), secret) {
				t.Errorf("GET %s Accept %q leaked a hidden field: %q", target, accept, rec.Body)
			}
		}
	}
}

func TestStructResultsAreJSONByDefault(t *testing.T) {
	type API struct {
		api.Specification
		Pointer func(context.Context) (*hiddenProfile, error)  `rest:"GET /pointer"`
		Double  func(context.Context) (**hiddenProfile, error) `rest:"GET /double"`
		Any     func(context.Context) (any, error)             `rest:"GET /any"`
	}
	h, err := rest.Handler(nil, API{
		Pointer: func(context.Context) (*hiddenProfile, error) { p := newHiddenProfile(); return &p, nil },
		Double: func(context.Context) (**hiddenProfile, error) {
			p := newHiddenProfile()
			q := &p
			return &q, nil
		},
		Any: func(context.Context) (any, error) { return newHiddenProfile(), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/pointer", "/double", "/any"} {
		for _, accept := range []string{"", "*/*"} {
			headers := []string{}
			if accept != "" {
				headers = append(headers, "Accept", accept)
			}
			rec := serve(t, h, "GET", target, "", headers...)
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("GET %s Accept %q: Content-Type %q", target, accept, ct)
			}
			if !strings.Contains(rec.Body.String(), `"title": "t"`) {
				t.Errorf("GET %s Accept %q: %q", target, accept, rec.Body)
			}
		}
	}
}

func TestPlainTextNeverDumpsAStruct(t *testing.T) {
	type API struct {
		api.Specification
		One  func(context.Context) (hiddenProfile, error)   `rest:"GET /one"`
		Many func(context.Context) ([]hiddenProfile, error) `rest:"GET /many"`
	}
	h, err := rest.Handler(nil, API{
		One:  func(context.Context) (hiddenProfile, error) { return newHiddenProfile(), nil },
		Many: func(context.Context) ([]hiddenProfile, error) { return []hiddenProfile{newHiddenProfile()}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/one", "/many"} {
		rec := serve(t, h, "GET", target, "", "Accept", "text/plain")
		if strings.Contains(rec.Body.String(), "{t ") || !strings.Contains(rec.Body.String(), `"title"`) {
			t.Errorf("GET %s as text/plain: %q", target, rec.Body)
		}
	}
}

func TestHiddenFieldsCannotBeSetByABody(t *testing.T) {
	type API struct {
		api.Specification
		Sign    func(context.Context, hiddenAccount) (string, error)      `rest:"POST /sign"`
		Pair    func(context.Context, hiddenAccount, int) (string, error) `rest:"POST /pair (account,n)"`
		Pointer func(context.Context, *hiddenAccount) (string, error)     `rest:"POST /pointer"`
	}
	leaked := func(a hiddenAccount) string {
		return fmt.Sprint(a.Name, a.Password, a.Nested.Secret, a.Pointer != nil && a.Pointer.Secret != "",
			len(a.Slice) > 0 && a.Slice[0].Secret != "")
	}
	h, err := rest.Handler(nil, API{
		Sign:    func(_ context.Context, a hiddenAccount) (string, error) { return leaked(a), nil },
		Pair:    func(_ context.Context, a hiddenAccount, _ int) (string, error) { return leaked(a), nil },
		Pointer: func(_ context.Context, a *hiddenAccount) (string, error) { return leaked(*a), nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	const body = `<Name>al</Name><Password>x</Password><Nested><Title>t</Title><Secret>x</Secret></Nested>` +
		`<Pointer><Secret>x</Secret></Pointer><Slice><Secret>x</Secret></Slice>`
	const want = "al" + "false false"
	for target, xml := range map[string]string{
		"/sign":    "<a>" + body + "</a>",
		"/pointer": "<a>" + body + "</a>",
		"/pair":    "<x><account>" + body + "</account><n>1</n></x>",
	} {
		rec := serve(t, h, "POST", target, xml, "Content-Type", "application/xml", "Accept", "text/plain")
		if rec.Code != 200 || rec.Body.String() != want {
			t.Errorf("XML POST %s: %d %q, want %q", target, rec.Code, rec.Body, want)
		}
	}
	rec := serve(t, h, "POST", "/sign", `{"name":"al","Password":"x","-":"x"}`, "Accept", "text/plain")
	if rec.Code != 200 || rec.Body.String() != want {
		t.Errorf("JSON POST /sign: %d %q, want %q", rec.Code, rec.Body, want)
	}
	if rec := serve(t, h, "POST", "/sign", "Password=x", "Content-Type", "text/plain"); rec.Code != http.StatusBadRequest {
		t.Errorf("text/plain into a struct: %d %q", rec.Code, rec.Body)
	}
}

type tenantFilter struct {
	Name   string `json:"name"`
	Tenant string `json:"-"`
	Tagged string `json:"-" rest:"tagged"`
	Dashed string `rest:"-"`
	Inner  struct {
		Kept  string `json:"kept"`
		Owner string `json:"-"`
	} `json:"inner"`
}

func TestHiddenFieldsAreNotQueryParameters(t *testing.T) {
	type API struct {
		api.Specification
		List func(context.Context, tenantFilter) (string, error) `rest:"GET /list?%v"`
	}
	var query string
	h, err := rest.Handler(nil, API{
		List: func(_ context.Context, f tenantFilter) (string, error) {
			return fmt.Sprint(f.Name, "|", f.Inner.Kept, "|", f.Tenant, f.Tagged, f.Dashed, f.Inner.Owner), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	const target = "/list?name=a&inner.kept=b&-=x&Tenant=x&tagged=x&Dashed=x&inner.-=x&inner.Owner=x&Inner.Owner=x"
	if rec := serve(t, h, "GET", target, ""); rec.Body.String() != "a|b|" {
		t.Errorf("GET %s: %d %q", target, rec.Code, rec.Body)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		h.ServeHTTP(w, r)
	}))
	defer srv.Close()
	client := api.Import[API](rest.API, srv.URL, srv.Client())
	filter := tenantFilter{Name: "a", Tenant: secret, Tagged: secret, Dashed: secret}
	filter.Inner.Owner = secret
	if _, err := client.List(context.Background(), filter); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(query, secret) {
		t.Errorf("the client put a hidden field on the wire: %q", query)
	}
}

// selfReferencing cycles through a pointer, which no encoder may follow forever.
type selfReferencing struct {
	Name   string           `json:"name"`
	Secret string           `json:"-"`
	Next   *selfReferencing `json:"next"`
}

func TestCyclicResultsDoNotOverflowEncoders(t *testing.T) {
	type API struct {
		api.Specification
		Loop func(context.Context) (*selfReferencing, error) `rest:"GET /loop"`
	}
	h, err := rest.Handler(nil, API{
		Loop: func(context.Context) (*selfReferencing, error) {
			loop := &selfReferencing{Name: "a", Secret: secret}
			loop.Next = loop
			return loop, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, accept := range []string{"application/json", "application/xml", "multipart/form-data", "text/plain"} {
		rec := serve(t, h, "GET", "/loop", "", "Accept", accept)
		// A multipart form is streamed, so its status has gone by the time
		// the cycle is found.
		if rec.Code != http.StatusInternalServerError && accept != "multipart/form-data" {
			t.Errorf("Accept %s: %d", accept, rec.Code)
		}
		if strings.Contains(rec.Body.String(), secret) {
			t.Errorf("Accept %s leaked a hidden field: %q", accept, rec.Body)
		}
	}
}
