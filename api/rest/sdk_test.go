package rest_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
)

type sdkThings struct {
	api.Specification

	List   func(context.Context) (string, error)                 `rest:"GET /things"`
	Get    func(context.Context, string) (string, error)         `rest:"GET /things/{id=%v}"`
	Make   func(context.Context, string) (string, error)         `rest:"POST /things"`
	Named  func(context.Context) (string, error)                 `rest:"GET /things/name"`
	Parts  func(context.Context, string) (string, error)         `rest:"GET /things/{id=%v}/parts"`
	Part   func(context.Context, string, string) (string, error) `rest:"GET /things/{id=%v}/parts/{part=%v}"`
	Inner  func(context.Context, string) (string, error)         `rest:"PUT /things/{id=%v}/parts/deep/down"`
	Hyphen func(context.Context) (string, error)                 `rest:"GET /do-something"`
	Deep   func(context.Context) (string, error)                 `rest:"GET /a/b/c"`
	Query  func(context.Context) (string, error)                 `rest:"QUERY /search"`

	Pair   func(context.Context, string, string) (string, error)         `rest:"GET /pairs/{a=%v}/{b=%v}"`
	Halves func(context.Context, string) (string, error)                 `rest:"PUT /pairs/{a=%v}"`
	Triple func(context.Context, string, string, string) (string, error) `rest:"POST /pairs/{a=%v}/{b=%v}/{c=%v}"`
	Within func(context.Context, string, string) (string, error)         `rest:"GET /pairs/{a=%v}/{b=%v}/inside"`
}

type sdkRoot struct {
	api.Specification

	Echo func(context.Context, string) (string, error) `rest:"POST /{s=%v}"`
}

const sdkDriver = `
import { API } from "./sdk.mjs";
import { API as Root } from "./root.mjs";

const calls = [];
const respond = (status, body) => async (url, opts) => {
  calls.push({ url, method: (opts && opts.method) || "GET", opts });
  return { status, body, json: async () => body };
};
const assert = (ok, label) => { if (!ok) { console.error("FAILED: " + label); process.exitCode = 1; } };
const last = () => calls[calls.length - 1];

const c = API("http://h", respond(200, "payload"));
assert(typeof c.things === "function", "a resource with children by value is callable");
assert(Object.keys(c).includes("things"), "resources are enumerable");

assert((await c.things.GET({ a: 1, b: "x y" })) === "payload", "GET resolves on a 200");
assert(last().url === "http://h/things?a=1&b=x+y", "GET sends the query: " + last().url);
await c.things.GET();
assert(last().url === "http://h/things", "GET with no query: " + last().url);
assert(last().opts.headers.Accept === "application/json", "GET asks for JSON");

assert((await c.things("5").GET({})) === "payload", "a path value");
assert(last().url === "http://h/things/5", "path value url: " + last().url);
await c.things("a/b c").GET();
assert(last().url === "http://h/things/a%2Fb%20c", "path values are escaped: " + last().url);
await c.things.name.GET();
assert(last().url === "http://h/things/name", "a child called name: " + last().url);
await c.things("5").parts.GET();
assert(last().url === "http://h/things/5/parts", "a literal under a value: " + last().url);
await c.things("5").parts("6").GET();
assert(last().url === "http://h/things/5/parts/6", "two values: " + last().url);
await c.things("5").parts.deep.down.PUT("x");
assert(last().url === "http://h/things/5/parts/deep/down", "nesting under a value: " + last().url);
await c.pairs("1")("2").GET();
assert(last().url === "http://h/pairs/1/2", "a value after a value: " + last().url);
await c.pairs("1").PUT("x");
assert(last().url === "http://h/pairs/1" && last().method === "PUT", "the first of two values is a resource too: " + last().url);
await c.pairs("1")("2")("3 4").POST("x");
assert(last().url === "http://h/pairs/1/2/3%204", "three values in a row: " + last().url);
await c.pairs("1")("2").inside.GET();
assert(last().url === "http://h/pairs/1/2/inside", "a literal under two values: " + last().url);
const five = c.pairs("5");
c.pairs("7")("8");
await five("6").GET();
assert(last().url === "http://h/pairs/5/6", "a later value leaves an earlier client alone: " + last().url);

for (const empty of ["", undefined, null]) {
  const before = calls.length;
  let threw = false;
  try { await c.things(empty).GET(); } catch (e) { threw = true; }
  assert(threw && calls.length === before, "an empty path value is not sent: " + JSON.stringify(empty));
}
await c.things(0).GET();
assert(last().url === "http://h/things/0", "zero is a path value: " + last().url);
await c.things(false).GET();
assert(last().url === "http://h/things/false", "false is a path value: " + last().url);

await c["do-something"].GET();
assert(last().url === "http://h/do-something", "a hyphen: " + last().url);
await c.a.b.c.GET();
assert(last().url === "http://h/a/b/c", "deep literals: " + last().url);

assert((await c.things.POST("x")) === "payload", "POST resolves on a 200");
assert(last().method === "POST" && last().opts.body === '"x"', "POST sends a body");
await c.search.QUERY("x");
assert(last().method === "POST" && last().opts.headers["X-HTTP-Method-Override"] === "QUERY", "QUERY goes by override");

assert((await API("http://h", respond(204)).things.POST("x")) === null, "204 is null");
let rejected = false;
await API("http://h", respond(500, "boom")).things.POST("x").catch(() => { rejected = true; });
assert(rejected, "a failure rejects");

globalThis.fetch = respond(200, "global");
assert((await API("http://h").things.POST("x")) === "global", "fetch defaults to the global one");

const root = Root("http://h", respond(200, "payload"));
assert((await root("a").POST("x")) === "payload", "a value at the root");
assert(last().url === "http://h/a", "root value url: " + last().url);
`

// TestGeneratedSDK runs the generated client in node when it is available; the
// checks on the text below do not need it.
func TestGeneratedSDK(t *testing.T) {
	sdk := func(impl any) []byte {
		h, err := rest.Handler(nil, impl)
		if err != nil {
			t.Fatal(err)
		}
		rec := serve(t, h, "GET", "/documentation", "", "Accept", "application/javascript")
		if rec.Code != 200 {
			t.Fatalf("SDK: %d %q", rec.Code, rec.Body)
		}
		return rec.Body.Bytes()
	}
	dir := t.TempDir()
	plain, root := sdk(sdkThings{}), sdk(sdkRoot{})
	for _, unwanted := range []string{"URLSearchParams(object)", "client.do-something", "API API"} {
		if strings.Contains(string(plain), unwanted) {
			t.Errorf("the SDK holds %q:\n%s", unwanted, plain)
		}
	}
	for name, data := range map[string][]byte{"sdk.mjs": plain, "root.mjs": root, "driver.mjs": []byte(sdkDriver)} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, node, filepath.Join(dir, "driver.mjs")).CombinedOutput()
	if err != nil {
		t.Errorf("%v\n%s\nSDK:\n%s", err, out, plain)
	}
}
