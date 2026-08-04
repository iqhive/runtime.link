---
name: testing-api-rest
description: How to runtime-test runtime.link's api/rest package (websockets, content-type decoding, generated HTML docs/examples pages) end-to-end on a local box.
---

# Testing `api/rest` at runtime

## Standing up a real server

`api/rest` is a library, so build a throwaway program in `/tmp` with a `go.mod`
that replaces the module with the local checkout:

```
module wstest
go 1.24
require runtime.link v0.0.0
replace runtime.link => /home/ubuntu/repos/runtime.link
```

Then `rest.Handler(nil, &impl)` + `http.ListenAndServe`. Declare endpoints on a
struct embedding `api.Specification` with `rest:"..."` tags.

Useful endpoint shapes:
- **Websocket**: a function returning a receive-only channel, e.g.
  `Stream func(context.Context) <-chan string \`rest:"GET /stream"\``.
  `host.go` routes recv-only channel results to `websocketServeHTTP` only when
  `Accept` is neither `text/event-stream` nor `application/json` — a browser or
  a real ws client gets the websocket, `curl` with `Accept: application/json`
  gets a JSON stream instead.
- **Body decoding**: `Echo func(ctx, T) string \`rest:"POST /echo"\``. Do NOT add
  argument rules like `(v)` if you want the plain body decoder — with rules the
  body is decoded into a generated mapping struct and `text/plain` bodies fail
  with `please provide valid 'text/plain'`.
- **Docs/examples pages**: implement `api.WithExamples`
  (`Example(ctx,name) (api.Example,bool)` and `Examples(ctx) (map[string][]string, error)`)
  on the impl struct; pages are then served at `GET /examples/{name}`.

## Websocket clients

`pip install websockets` and use `websockets.connect("ws://127.0.0.1:PORT/path")`.
Its parser is strict RFC6455, so it catches malformed extended-length frames
(you get `ConnectionClosedError ... 1002 (protocol error) invalid opcode`).
Always test payloads of <126, 126..65535 and >65535 bytes — the three
length encodings are separate code paths.

Inbound (client→server) frame handling is **not reachable** through
`rest.Handler`: it passes an invalid recv value to `websocketServeHTTP`. To test
unmasking / frame-size limits, write a white-box test in package `rest` that
calls `websocketServeHTTP(ctx, r, w, sendChan, recvChan)` from an
`httptest.NewServer` handler and drive it over a raw `net.Dial` socket with
hand-built masked frames.

The Go **client** side (`websocketOpen`, used via `api.Import`/link.go) may be
dead at runtime: it requires `resp.Body` to implement `Hijack()`, but Go's
`net/http` client returns `*http.readWriteCloserBody`, which is not hijackable,
so it returns right after the handshake. If a client-side websocket test never
connects, this is likely why — test the server side instead and say the client
side is unverifiable.

## Docs/examples page XSS testing

Server-side escaping is not the whole story: the generated page runs

```js
document.querySelectorAll(".markdown").forEach(el => {
  el.innerHTML = marked.parse(dedent(el.textContent));
});
```

so anything rendered into `<div class="markdown">` (example `Story`, `Tests`,
step `Note`) is un-escaped and re-injected as HTML client-side — server-side
`html.EscapeString` there is ineffective. Always check XSS payloads **in a real
browser**, not just with `curl`; a `curl` response full of `&lt;` can still
execute. Use a payload with a visible side effect (e.g.
`<img src=x onerror="...insertAdjacentHTML(...)">`) rather than `alert()`, since
headless browsers auto-dismiss dialogs and screenshots then prove nothing.
Fields rendered into `<pre>` (sample URL, request/response bodies, errors) are
not markdown-processed and stay safely escaped.

## Proving a fix actually fixes something

Re-run each check against the pre-fix code with
`git checkout origin/main -- api/rest/<file>` (rebuild the /tmp program), then
`git checkout HEAD -- api/rest/<file>`. Beware: `git stash -- <file>` exits 0
with "No local changes to save" when the file is unmodified, so a
`git stash || git checkout` chain silently does nothing — use `git checkout`
directly. Also `pkill` the old binary before starting a new one; otherwise the
new process fails to bind the port and you silently keep testing the old build.

## Known pre-existing failures (ignore)

`go test ./api/rest -run TestParams` and `go test ./api -run TestStructure`
fail on main under Go 1.26.5.

## Devin Secrets Needed

None — everything runs locally with no credentials.
