package rest_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"runtime.link/api"
	"runtime.link/api/rest"
)

type securityUser struct {
	Name         string `json:"name"`
	PasswordHash string `json:"-"`
	TenantID     string `json:"-"`
	IsAdmin      bool   `json:"-"`
	private      string
}

type securityAPI struct {
	api.Specification
	Get    func(context.Context, string) (securityUser, error) `rest:"GET /users/{id=%v}"`
	Delete func(context.Context, string) error                 `rest:"DELETE /users/{id=%v}"`
	Update func(context.Context, securityUser) (string, error) `rest:"PUT /me"`
	Search func(context.Context, securityUser) (string, error) `rest:"GET /search?%v"`
	Ratio  func(context.Context, float64) (string, error)      `rest:"GET /ratio?x=%v"`
	Watch  func(context.Context) (<-chan string, error)        `rest:"GET /watch"`
}

func newSecurityServer(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var deleted []string
	h, err := rest.Handler(nil, securityAPI{
		Get: func(_ context.Context, id string) (securityUser, error) {
			return securityUser{Name: "alice", PasswordHash: "$2a$secret", TenantID: "t1", private: "unexported"}, nil
		},
		Delete: func(_ context.Context, id string) error { deleted = append(deleted, id); return nil },
		Update: func(_ context.Context, u securityUser) (string, error) {
			return fmt.Sprint(u.Name, u.IsAdmin, u.TenantID), nil
		},
		Search: func(_ context.Context, u securityUser) (string, error) {
			return fmt.Sprint(u.Name, u.IsAdmin, u.TenantID), nil
		},
		Ratio: func(_ context.Context, x float64) (string, error) {
			if x < 0 || x > 1 {
				return "", errors.New("out of range")
			}
			return "ok", nil
		},
		Watch: func(ctx context.Context) (<-chan string, error) {
			c := make(chan string, 1)
			c <- "private-event"
			return c, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, &deleted
}

func securityDo(t *testing.T, method, url, body string, header ...string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func TestMethodNameInPathIsNotARoute(t *testing.T) {
	srv, deleted := newSecurityServer(t)
	securityDo(t, "GET", srv.URL+"/users/42/DELETE", "")
	securityDo(t, "DELETE", srv.URL+"/users/", "")
	client := api.Import[securityAPI](rest.API, srv.URL, srv.Client())
	client.Get(context.Background(), "43/DELETE")
	if len(*deleted) != 0 {
		t.Fatalf("Delete was reached through the path: %v", *deleted)
	}
	if resp, _ := securityDo(t, "DELETE", srv.URL+"/users/44", ""); resp.StatusCode >= 300 || len(*deleted) != 1 || (*deleted)[0] != "44" {
		t.Fatalf("DELETE /users/44 -> %d, deleted %v", resp.StatusCode, *deleted)
	}
}

func TestEscapedSlashStaysInOneSegment(t *testing.T) {
	srv, _ := newSecurityServer(t)
	resp, _ := securityDo(t, "GET", srv.URL+"/users/a%2Fb", "", "Accept", "application/json")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /users/a%%2Fb -> %d", resp.StatusCode)
	}
}

func TestHiddenFieldsNeverLeaveInAResponse(t *testing.T) {
	srv, _ := newSecurityServer(t)
	for _, accept := range []string{"application/json", "application/xml", "text/plain"} {
		_, body := securityDo(t, "GET", srv.URL+"/users/1", "", "Accept", accept)
		if !strings.Contains(body, "alice") {
			t.Errorf("Accept %s: name missing from %q", accept, body)
		}
		for _, secret := range []string{"$2a$secret", "t1", "unexported"} {
			if strings.Contains(body, secret) {
				t.Errorf("Accept %s: %q leaked in %q", accept, secret, body)
			}
		}
	}
}

func TestHiddenFieldsCannotBeSetByARequest(t *testing.T) {
	srv, _ := newSecurityServer(t)
	_, body := securityDo(t, "PUT", srv.URL+"/me",
		"<securityUser><Name>x</Name><IsAdmin>true</IsAdmin><TenantID>victim</TenantID></securityUser>",
		"Content-Type", "application/xml", "Accept", "application/json")
	if strings.Contains(body, "true") || strings.Contains(body, "victim") {
		t.Errorf("XML body set hidden fields: %q", body)
	}
	_, body = securityDo(t, "GET", srv.URL+"/search?name=x&-=victim&IsAdmin=true&TenantID=victim", "", "Accept", "application/json")
	if strings.Contains(body, "true") || strings.Contains(body, "victim") {
		t.Errorf("query set hidden fields: %q", body)
	}
}

func TestDecodedBodiesAreLimited(t *testing.T) {
	srv, _ := newSecurityServer(t)
	huge := `{"name":"` + strings.Repeat("a", 32<<20) + `"}`
	resp, _ := securityDo(t, "PUT", srv.URL+"/me", huge, "Content-Type", "application/json")
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body -> %d, want 413", resp.StatusCode)
	}
}

func TestBodyWithoutDecoderIsRefused(t *testing.T) {
	srv, _ := newSecurityServer(t)
	resp, _ := securityDo(t, "PUT", srv.URL+"/me", "x", "Content-Type", "multipart/form-data")
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("multipart body -> %d, want 415", resp.StatusCode)
	}
}

func TestRootRedirectStaysOnThisServer(t *testing.T) {
	srv, _ := newSecurityServer(t)
	for _, path := range []string{"//evil.example/", "/\\evil.example/"} {
		resp, _ := securityDo(t, "GET", srv.URL+path, "")
		if location := resp.Header.Get("Location"); strings.HasPrefix(location, "//") || strings.HasPrefix(location, "/\\") {
			t.Errorf("GET %s redirected off this server, to %q", path, location)
		}
	}
}

func TestNonFiniteNumbersAreNotParameters(t *testing.T) {
	srv, _ := newSecurityServer(t)
	for _, x := range []string{"NaN", "Inf", "-Inf"} {
		resp, body := securityDo(t, "GET", srv.URL+"/ratio?x="+x, "", "Accept", "application/json")
		if resp.StatusCode < 400 {
			t.Errorf("x=%s -> %d %q", x, resp.StatusCode, body)
		}
	}
}

func websocketHandshake(t *testing.T, srv *httptest.Server, origin string) string {
	t.Helper()
	conn, err := net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	fmt.Fprintf(conn, "GET /watch HTTP/1.1\r\nHost: %s\r\nOrigin: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n", host, origin)
	status, _ := bufio.NewReader(conn).ReadString('\n')
	return strings.TrimSpace(status)
}

func TestWebsocketOrigin(t *testing.T) {
	srv, _ := newSecurityServer(t)
	if status := websocketHandshake(t, srv, "https://evil.example"); !strings.Contains(status, "403") {
		t.Errorf("cross-origin handshake -> %q, want 403", status)
	}
	if status := websocketHandshake(t, srv, srv.URL); !strings.Contains(status, "101") {
		t.Errorf("same-origin handshake -> %q, want 101", status)
	}
}

func TestClientKeepsCredentialsOnItsHost(t *testing.T) {
	var leaked string
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = r.Header.Get("Authorization")
		w.Write([]byte(`{"name":"x"}`))
	}))
	defer elsewhere.Close()
	// 127.0.0.1 and localhost are different hosts to the client.
	elsewhereURL := strings.Replace(elsewhere.URL, "127.0.0.1", "localhost", 1)
	var seen string
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		http.Redirect(w, r, elsewhereURL+r.URL.Path, http.StatusFound)
	}))
	defer origin.Close()
	client := api.Import[securityAPI](rest.API, origin.URL, rest.Header("Authorization", "Bearer secret"))
	client.Get(context.Background(), "1")
	if seen != "Bearer secret" {
		t.Fatalf("credential not sent to its own host: %q", seen)
	}
	if leaked != "" {
		t.Fatalf("credential followed the redirect to another host: %q", leaked)
	}
}

// refuseAdmin is an Auth that authenticates anyone, and authorizes no one to
// call the functions named Secret.
type refuseAdmin struct{}

func (refuseAdmin) Authenticate(ctx context.Context, _ *http.Request, _ api.Function) (context.Context, error) {
	return ctx, nil
}
func (refuseAdmin) Authorize(_ context.Context, _ *http.Request, fn api.Function, _ []reflect.Value) error {
	if fn.Name == "Secret" {
		return errors.New("forbidden")
	}
	return nil
}
func (refuseAdmin) Redact(_ context.Context, err error) error { return err }

type refusedAPI struct {
	api.Specification
	Public func(context.Context) (string, error) `rest:"GET /public"`
	Admin  struct {
		Secret func(context.Context, securityUser) error `rest:"POST /admin/secret"`
	}
}

func TestDocumentationOmitsRefusedFunctions(t *testing.T) {
	var impl refusedAPI
	impl.Public = func(context.Context) (string, error) { return "ok", nil }
	impl.Admin.Secret = func(context.Context, securityUser) error { return nil }
	h, err := rest.Handler(refuseAdmin{}, impl)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	_, body := securityDo(t, "GET", srv.URL+"/documentation", "", "Accept", "application/json")
	if !strings.Contains(body, "/public") || strings.Contains(body, "/admin/secret") {
		t.Errorf("OpenAPI document shows a refused function, or misses an allowed one: %s", body)
	}
	resp, body := securityDo(t, "GET", srv.URL+"/admin/secret?method=POST", "", "Accept", "application/schema+json")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("form schema of a refused function -> %d %q", resp.StatusCode, body)
	}
}
