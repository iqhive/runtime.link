package rest_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/rest"
)

// validatedCode is a string that validates itself when read from JSON, and has
// no UnmarshalText.
type validatedCode string

func (c *validatedCode) UnmarshalJSON(b []byte) error {
	var text string
	if err := json.Unmarshal(b, &text); err != nil {
		return err
	}
	if !strings.HasPrefix(text, "c-") {
		return fmt.Errorf("not a code")
	}
	*c = validatedCode(strings.ToUpper(text))
	return nil
}

// validatedHex is a string that validates itself through fmt.Scanner alone.
type validatedHex string

func (h *validatedHex) Scan(state fmt.ScanState, verb rune) error {
	token, err := state.Token(true, nil)
	if err != nil {
		return err
	}
	if _, err := hex.DecodeString(string(token)); err != nil || len(token) == 0 {
		return fmt.Errorf("not hex")
	}
	*h = validatedHex(strings.ToLower(string(token)))
	return nil
}

// validatedTags is a list that is one value to its text form, which it checks.
type validatedTags []string

func (tags *validatedTags) UnmarshalText(b []byte) error {
	*tags = strings.Split(string(b), "+")
	for _, tag := range *tags {
		if tag == "" {
			return fmt.Errorf("empty tag")
		}
	}
	return nil
}

type validatedFilter struct {
	Email validatedEmail   `json:"email"`
	Code  *validatedCode   `json:"code"`
	Hexes []validatedHex   `json:"hex"`
	Also  []validatedEmail `json:"also"`
}

type validatedThing struct {
	Email validatedEmail `json:"email" xml:"email"`
	Note  string         `json:"note" xml:"note"`
}

type validatedAPI struct {
	api.Specification

	Query    func(context.Context, validatedEmail) (string, error)              `rest:"GET /q?e=%v"`
	Path     func(context.Context, validatedEmail) (string, error)              `rest:"GET /p/{e=%v}"`
	Rest     func(context.Context, validatedEmail) (string, error)              `rest:"GET /r/{e*=%v}"`
	Pointer  func(context.Context, *validatedEmail) (string, error)             `rest:"GET /ptr?e=%v"`
	Double   func(context.Context, **validatedEmail) (string, error)            `rest:"GET /pptr?e=%v"`
	List     func(context.Context, []validatedEmail) (string, error)            `rest:"GET /list?e=%v"`
	PList    func(context.Context, []*validatedEmail) (string, error)           `rest:"GET /plist?e=%v"`
	ListPtr  func(context.Context, *[]validatedEmail) (string, error)           `rest:"GET /listptr?e=%v"`
	PathList func(context.Context, []validatedEmail) (string, error)            `rest:"GET /pl/{e=%v}"`
	Map      func(context.Context, map[string]validatedEmail) (string, error)   `rest:"GET /map?%v"`
	MapKey   func(context.Context, map[validatedEmail]string) (string, error)   `rest:"GET /mapkey?%v"`
	MapList  func(context.Context, map[string][]validatedEmail) (string, error) `rest:"GET /maplist?%v"`
	Expand   func(context.Context, validatedFilter) (string, error)             `rest:"GET /expand?%v"`
	Nested   func(context.Context, validatedThing) (string, error)              `rest:"POST /nested/{Email}"`
	Plain    func(context.Context, validatedEmail) (string, error)              `rest:"POST(text/plain) /plain"`
	JSON     func(context.Context, validatedEmail) (string, error)              `rest:"POST /json"`
	Struct   func(context.Context, validatedThing) (string, error)              `rest:"POST /struct"`
	Pair     func(context.Context, validatedEmail, int) (string, error)         `rest:"POST /pair"`
	Mapped   func(context.Context, validatedEmail, int) (string, error)         `rest:"POST /mapped (email,n)"`

	Code      func(context.Context, validatedCode) (string, error)   `rest:"GET /code?c=%v"`
	CodePath  func(context.Context, validatedCode) (string, error)   `rest:"GET /code/{c=%v}"`
	CodePtr   func(context.Context, *validatedCode) (string, error)  `rest:"GET /codeptr?c=%v"`
	CodeList  func(context.Context, []validatedCode) (string, error) `rest:"GET /codes?c=%v"`
	CodePlain func(context.Context, validatedCode) (string, error)   `rest:"POST(text/plain) /codeplain"`

	Hex      func(context.Context, validatedHex) (string, error)   `rest:"GET /hex?h=%v"`
	HexPath  func(context.Context, validatedHex) (string, error)   `rest:"GET /hex/{h=%v}"`
	HexList  func(context.Context, []validatedHex) (string, error) `rest:"GET /hexes?h=%v"`
	HexPlain func(context.Context, validatedHex) (string, error)   `rest:"POST(text/plain) /hexplain"`

	Tags    func(context.Context, validatedTags) (string, error) `rest:"GET /tags?t=%v"`
	Port    func(context.Context, udpPort) (int, error)          `rest:"GET /port?v=%v"`
	PortTxt func(context.Context, udpPort) (int, error)          `rest:"POST(text/plain) /port"`

	String  func(context.Context, string) (string, error)   `rest:"GET /string?v=%v"`
	Strings func(context.Context, []string) (string, error) `rest:"GET /strings?v=%v"`
}

func newValidatedAPI() validatedAPI {
	email := func(_ context.Context, e validatedEmail) (string, error) { return "email:" + string(e), nil }
	code := func(_ context.Context, c validatedCode) (string, error) { return "code:" + string(c), nil }
	hexed := func(_ context.Context, h validatedHex) (string, error) { return "hex:" + string(h), nil }
	return validatedAPI{
		Query: email, Path: email, Rest: email, Plain: email, JSON: email,
		Pointer: func(_ context.Context, e *validatedEmail) (string, error) {
			if e == nil {
				return "absent", nil
			}
			return "email:" + string(*e), nil
		},
		Double: func(_ context.Context, e **validatedEmail) (string, error) {
			if e == nil {
				return "absent", nil
			}
			return "email:" + string(**e), nil
		},
		List: func(_ context.Context, list []validatedEmail) (string, error) { return fmt.Sprintf("%q", list), nil },
		PList: func(_ context.Context, list []*validatedEmail) (string, error) {
			var out []string
			for _, e := range list {
				if e == nil {
					out = append(out, "<nil>")
					continue
				}
				out = append(out, string(*e))
			}
			return fmt.Sprintf("%q", out), nil
		},
		ListPtr: func(_ context.Context, list *[]validatedEmail) (string, error) {
			if list == nil {
				return "absent", nil
			}
			return fmt.Sprintf("%q", *list), nil
		},
		PathList: func(_ context.Context, list []validatedEmail) (string, error) { return fmt.Sprintf("%q", list), nil },
		Map: func(_ context.Context, m map[string]validatedEmail) (string, error) {
			var out []string
			for key, value := range m {
				out = append(out, key+"="+string(value))
			}
			sort.Strings(out)
			return strings.Join(out, ","), nil
		},
		MapKey: func(_ context.Context, m map[validatedEmail]string) (string, error) {
			var out []string
			for key, value := range m {
				out = append(out, string(key)+"="+value)
			}
			sort.Strings(out)
			return strings.Join(out, ","), nil
		},
		MapList: func(_ context.Context, m map[string][]validatedEmail) (string, error) {
			var out []string
			for key, value := range m {
				out = append(out, fmt.Sprintf("%s=%q", key, value))
			}
			sort.Strings(out)
			return strings.Join(out, ","), nil
		},
		Expand: func(_ context.Context, f validatedFilter) (string, error) {
			out := "email:" + string(f.Email)
			if f.Code != nil {
				out += " code:" + string(*f.Code)
			}
			return out + fmt.Sprintf(" hex:%q also:%q", f.Hexes, f.Also), nil
		},
		Nested: func(_ context.Context, th validatedThing) (string, error) {
			return "email:" + string(th.Email) + " note:" + th.Note, nil
		},
		Struct: func(_ context.Context, th validatedThing) (string, error) {
			return "email:" + string(th.Email) + " note:" + th.Note, nil
		},
		Pair: func(_ context.Context, e validatedEmail, n int) (string, error) {
			return fmt.Sprint("email:", e, " n:", n), nil
		},
		Mapped: func(_ context.Context, e validatedEmail, n int) (string, error) {
			return fmt.Sprint("email:", e, " n:", n), nil
		},

		Code: code, CodePath: code, CodePlain: code,
		CodePtr: func(_ context.Context, c *validatedCode) (string, error) {
			if c == nil {
				return "absent", nil
			}
			return "code:" + string(*c), nil
		},
		CodeList: func(_ context.Context, list []validatedCode) (string, error) { return fmt.Sprintf("%q", list), nil },

		Hex: hexed, HexPath: hexed, HexPlain: hexed,
		HexList: func(_ context.Context, list []validatedHex) (string, error) { return fmt.Sprintf("%q", list), nil },

		Tags:    func(_ context.Context, tags validatedTags) (string, error) { return strings.Join(tags, "|"), nil },
		Port:    func(_ context.Context, p udpPort) (int, error) { return p.n, nil },
		PortTxt: func(_ context.Context, p udpPort) (int, error) { return p.n, nil },

		String:  func(_ context.Context, s string) (string, error) { return "string:" + s, nil },
		Strings: func(_ context.Context, list []string) (string, error) { return fmt.Sprintf("%q", list), nil },
	}
}

type validatedCase struct {
	method, target, body, ctype string

	status int
	want   string // the body of a 200
}

func runValidatedCases(t *testing.T, cases []validatedCase) {
	t.Helper()
	h, err := rest.Handler(nil, newValidatedAPI())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		headers := []string{"Accept", "text/plain"}
		if tc.ctype != "" {
			headers = append(headers, "Content-Type", tc.ctype)
		}
		rec := serve(t, h, tc.method, tc.target, tc.body, headers...)
		if rec.Code != tc.status {
			t.Errorf("%s %s %q: %d %q, want %d", tc.method, tc.target, tc.body, rec.Code, rec.Body, tc.status)
			continue
		}
		if tc.status == http.StatusOK && rec.Body.String() != tc.want {
			t.Errorf("%s %s %q: %q, want %q", tc.method, tc.target, tc.body, rec.Body, tc.want)
		}
	}
}

// TestStringsThatValidateTheirTextAreNeverTakenAsSent checks every way a value
// reaches a function: a string type whose UnmarshalText refuses a value must
// never arrive holding it, as a handler that trusts its argument types has no
// other line of defence.
func TestStringsThatValidateTheirTextAreNeverTakenAsSent(t *testing.T) {
	runValidatedCases(t, []validatedCase{
		{"GET", "/q?e=nonsense", "", "", 400, ""},
		{"GET", "/q?e=A@B", "", "", 200, "email:a@b"},
		{"GET", "/p/nonsense", "", "", 400, ""},
		{"GET", "/p/A@B", "", "", 200, "email:a@b"},
		{"GET", "/r/non/sense", "", "", 400, ""},
		{"GET", "/r/A@B/c", "", "", 200, "email:a@b/c"},
		{"GET", "/ptr?e=nonsense", "", "", 400, ""},
		{"GET", "/ptr?e=A@B", "", "", 200, "email:a@b"},
		{"GET", "/pptr?e=nonsense", "", "", 400, ""},
		{"GET", "/pptr?e=A@B", "", "", 200, "email:a@b"},
		{"GET", "/list?e=A@B&e=nonsense", "", "", 400, ""},
		{"GET", "/list?e[]=A@B&e[]=nonsense", "", "", 400, ""},
		{"GET", "/list?e=A@B&e=C@D", "", "", 200, `["a@b" "c@d"]`},
		{"GET", "/plist?e=A@B&e=nonsense", "", "", 400, ""},
		{"GET", "/plist?e=A@B&e=C@D", "", "", 200, `["a@b" "c@d"]`},
		{"GET", "/listptr?e=A@B&e=nonsense", "", "", 400, ""},
		{"GET", "/listptr?e=A@B", "", "", 200, `["a@b"]`},
		{"GET", "/pl/A@B,nonsense", "", "", 400, ""},
		{"GET", "/pl/A@B,C@D", "", "", 200, `["a@b" "c@d"]`},
		{"GET", "/map?x=nonsense", "", "", 400, ""},
		{"GET", "/map?x=A@B&y=C@D", "", "", 200, "x=a@b,y=c@d"},
		{"GET", "/mapkey?nonsense=1", "", "", 400, ""},
		{"GET", "/mapkey?A@B=1", "", "", 200, "a@b=1"},
		{"GET", "/maplist?x=A@B&x=nonsense", "", "", 400, ""},
		{"GET", "/maplist?x=A@B&x=C@D", "", "", 200, `x=["a@b" "c@d"]`},
		{"GET", "/expand?email=nonsense", "", "", 400, ""},
		{"GET", "/expand?also=A@B&also=nonsense", "", "", 400, ""},
		{"GET", "/expand?email=A@B&also=C@D", "", "", 200, `email:a@b hex:[] also:["c@d"]`},
		{"POST", "/nested/nonsense", `{"note":"n"}`, "", 400, ""},
		{"POST", "/nested/A@B", `{"note":"n"}`, "", 200, "email:a@b note:n"},
		{"POST", "/nested/A@B", `{"note":"n","email":"nonsense"}`, "", 400, ""},
		{"POST", "/plain", "nonsense", "text/plain", 400, ""},
		{"POST", "/plain", "A@B", "text/plain", 200, "email:a@b"},
		{"POST", "/json", `"nonsense"`, "", 400, ""},
		{"POST", "/json", `"A@B"`, "", 200, "email:a@b"},
		{"POST", "/json", "nonsense", "text/plain", 400, ""},
		{"POST", "/json", "A@B", "text/plain", 200, "email:a@b"},
		{"POST", "/json", "<validatedEmail>nonsense</validatedEmail>", "application/xml", 400, ""},
		{"POST", "/json", "<validatedEmail>A@B</validatedEmail>", "application/xml", 200, "email:a@b"},
		{"POST", "/struct", `{"email":"nonsense"}`, "", 400, ""},
		{"POST", "/struct", `{"email":"A@B","note":"n"}`, "", 200, "email:a@b note:n"},
		{"POST", "/struct", "<validatedThing><email>nonsense</email></validatedThing>", "application/xml", 400, ""},
		{"POST", "/struct", "<validatedThing><email>A@B</email><note>n</note></validatedThing>", "application/xml", 200, "email:a@b note:n"},
		{"POST", "/pair", `["nonsense",1]`, "", 400, ""},
		{"POST", "/pair", `["A@B",1]`, "", 200, "email:a@b n:1"},
		{"POST", "/mapped", `{"email":"nonsense","n":1}`, "", 400, ""},
		{"POST", "/mapped", `{"email":"A@B","n":1}`, "", 200, "email:a@b n:1"},
	})
}

// TestStringsThatValidateTheirJSONOrScanAreAsked checks the string types that
// read themselves some other way than UnmarshalText: being a string by kind
// must not let a path, query or text body skip the only check they have.
func TestStringsThatValidateTheirJSONOrScanAreAsked(t *testing.T) {
	runValidatedCases(t, []validatedCase{
		{"GET", "/code?c=nonsense", "", "", 400, ""},
		{"GET", "/code?c=c-1", "", "", 200, "code:C-1"},
		// What looks like a number or a boolean is still a string to a
		// string type, and is checked as one.
		{"GET", "/code?c=12", "", "", 400, ""},
		{"GET", "/code?c=true", "", "", 400, ""},
		// Any text is a string to hand over, the characters JSON has its
		// own way of writing included.
		{"GET", "/code?c=c-%22%5C", "", "", 200, "code:C-\"\\"},
		{"GET", "/code?c=c-%07%1F", "", "", 200, "code:C-\a\x1f"},
		{"GET", "/code/nonsense", "", "", 400, ""},
		{"GET", "/code/c-1", "", "", 200, "code:C-1"},
		{"GET", "/codeptr?c=nonsense", "", "", 400, ""},
		{"GET", "/codeptr?c=c-1", "", "", 200, "code:C-1"},
		{"GET", "/codeptr", "", "", 200, "absent"},
		{"GET", "/codes?c=c-1&c=nonsense", "", "", 400, ""},
		{"GET", "/codes?c=c-1&c=c-2", "", "", 200, `["C-1" "C-2"]`},
		{"POST", "/codeplain", "nonsense", "text/plain", 400, ""},
		{"POST", "/codeplain", "c-1", "text/plain", 200, "code:C-1"},
		{"POST", "/codeplain", `"nonsense"`, "application/json", 400, ""},
		{"POST", "/codeplain", `"c-1"`, "application/json", 200, "code:C-1"},
		{"GET", "/expand?code=nonsense", "", "", 400, ""},
		{"GET", "/expand?code=c-1", "", "", 200, `email: code:C-1 hex:[] also:[]`},

		{"GET", "/hex?h=nonsense", "", "", 400, ""},
		{"GET", "/hex?h=BEEF", "", "", 200, "hex:beef"},
		{"GET", "/hex/nonsense", "", "", 400, ""},
		{"GET", "/hex/BEEF", "", "", 200, "hex:beef"},
		{"GET", "/hexes?h=BEEF&h=nonsense", "", "", 400, ""},
		{"GET", "/hexes?h=BEEF&h=00", "", "", 200, `["beef" "00"]`},
		{"POST", "/hexplain", "nonsense", "text/plain", 400, ""},
		{"POST", "/hexplain", "BEEF", "text/plain", 200, "hex:beef"},
		{"GET", "/expand?hex=BEEF&hex=nonsense", "", "", 400, ""},
		{"GET", "/expand?hex=BEEF", "", "", 200, `email: hex:["beef"] also:[]`},
	})
}

// TestScannersReadAllOfTheirText checks that a value read by fmt.Scanner is the
// whole of what was sent: a Scan that stops at the first character it does not
// know would otherwise accept any value that starts well.
func TestScannersReadAllOfTheirText(t *testing.T) {
	runValidatedCases(t, []validatedCase{
		{"GET", "/port?v=8080", "", "", 200, "8080"},
		{"GET", "/port?v=8080abc", "", "", 400, ""},
		{"GET", "/port?v=8080%209090", "", "", 400, ""},
		{"GET", "/port?v=abc", "", "", 400, ""},
		{"GET", "/hex?h=BEEF%20nonsense", "", "", 400, ""},
		// The newline that ends a line of text is not part of the value.
		{"POST", "/port", "8080\n", "text/plain", 200, "8080"},
		{"POST", "/port", "8080", "text/plain", 200, "8080"},
		{"POST", "/port", "8080 abc\n", "text/plain", 400, ""},
		{"POST", "/hexplain", "BEEF\n", "text/plain", 200, "hex:beef"},
		{"POST", "/hexplain", "BEEF nonsense", "text/plain", 400, ""},
	})
}

// TestListsThatReadTheirOwnTextAreOneValue checks that a slice type with an
// UnmarshalText is read by it: read element by element instead, nothing the
// type checks would be checked.
func TestListsThatReadTheirOwnTextAreOneValue(t *testing.T) {
	runValidatedCases(t, []validatedCase{
		{"GET", "/tags?t=a%2Bb", "", "", 200, "a|b"},
		{"GET", "/tags?t=a%2B", "", "", 400, ""},
	})
}

// TestEmptyValuesOfTypesThatValidate pins what an empty value means. For one
// parameter it is the parameter left out, which a function is always open to,
// so the zero value arrives. Where it is a value that was sent, as an element
// of a list or what an optional pointer points to, the type decides.
func TestEmptyValuesOfTypesThatValidate(t *testing.T) {
	runValidatedCases(t, []validatedCase{
		{"GET", "/q", "", "", 200, "email:"},
		{"GET", "/q?e=", "", "", 200, "email:"},
		{"GET", "/ptr", "", "", 200, "absent"},
		{"GET", "/ptr?e=", "", "", 400, ""},
		{"GET", "/code", "", "", 200, "code:"},
		{"GET", "/code?c=", "", "", 200, "code:"},
		{"GET", "/hex?h=", "", "", 200, "hex:"},
		{"GET", "/list", "", "", 200, `[]`},
		{"GET", "/list?e=&e=A@B", "", "", 400, ""},
		{"GET", "/list?e=A@B&e=", "", "", 400, ""},
		{"GET", "/plist?e=&e=A@B", "", "", 400, ""},
		{"GET", "/pl/A@B,,C@D", "", "", 400, ""},
		{"GET", "/codes?c=&c=c-1", "", "", 400, ""},
		{"GET", "/hexes?h=&h=BEEF", "", "", 400, ""},
		{"GET", "/maplist?x=&x=A@B", "", "", 400, ""},
		{"GET", "/map?x=", "", "", 400, ""},
		{"GET", "/expand?also=&also=A@B", "", "", 400, ""},
		{"POST", "/plain", "", "text/plain", 400, ""},
		// Plain strings have nothing to check, an empty one is a value.
		{"GET", "/string?v=", "", "", 200, "string:"},
		{"GET", "/string?v=Nonsense", "", "", 200, "string:Nonsense"},
		{"GET", "/strings?v=&v=x&v=", "", "", 200, `["" "x" ""]`},
	})
}

// TestValidatedStringsRoundTrip checks that the Go client and the server still
// agree on these types once the server checks them.
func TestValidatedStringsRoundTrip(t *testing.T) {
	h, err := rest.Handler(nil, newValidatedAPI())
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	client := api.Import[validatedAPI](rest.API, srv.URL, srv.Client())
	ctx := context.Background()
	email := validatedEmail("A@B")
	for name, call := range map[string]func() (string, error){
		"query":   func() (string, error) { return client.Query(ctx, "A@B") },
		"path":    func() (string, error) { return client.Path(ctx, "A@B") },
		"pointer": func() (string, error) { return client.Pointer(ctx, &email) },
		"plain":   func() (string, error) { return client.Plain(ctx, "A@B") },
		"json":    func() (string, error) { return client.JSON(ctx, "A@B") },
	} {
		if got, err := call(); err != nil || got != "email:a@b" {
			t.Errorf("%s: %q, %v", name, got, err)
		}
	}
	for name, call := range map[string]func() (string, error){
		"query":   func() (string, error) { return client.Query(ctx, "nonsense") },
		"path":    func() (string, error) { return client.Path(ctx, "nonsense") },
		"plain":   func() (string, error) { return client.Plain(ctx, "nonsense") },
		"json":    func() (string, error) { return client.JSON(ctx, "nonsense") },
		"list":    func() (string, error) { return client.List(ctx, []validatedEmail{"A@B", "nonsense"}) },
		"code":    func() (string, error) { return client.Code(ctx, "nonsense") },
		"hex":     func() (string, error) { return client.Hex(ctx, "nonsense") },
		"mapped":  func() (string, error) { return client.Mapped(ctx, "nonsense", 1) },
		"pair":    func() (string, error) { return client.Pair(ctx, "nonsense", 1) },
		"nested":  func() (string, error) { return client.Nested(ctx, validatedThing{Email: "nonsense"}) },
		"expand":  func() (string, error) { return client.Expand(ctx, validatedFilter{Email: "nonsense"}) },
		"mapkey":  func() (string, error) { return client.MapKey(ctx, map[validatedEmail]string{"nonsense": "x"}) },
		"maplist": func() (string, error) { return client.MapList(ctx, map[string][]validatedEmail{"x": {"nonsense"}}) },
	} {
		if got, err := call(); err == nil {
			t.Errorf("%s: an invalid value reached the function, which answered %q", name, got)
		}
	}
	if got, err := client.List(ctx, []validatedEmail{"A@B", "C@D"}); err != nil || got != `["a@b" "c@d"]` {
		t.Errorf("list: %q, %v", got, err)
	}
	if got, err := client.Code(ctx, "c-1"); err != nil || got != "code:C-1" {
		t.Errorf("code: %q, %v", got, err)
	}
	if got, err := client.Hex(ctx, "BEEF"); err != nil || got != "hex:beef" {
		t.Errorf("hex: %q, %v", got, err)
	}
}
