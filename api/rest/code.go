package rest

import (
	"bytes"
	_ "embed"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/iqhive/runtime.link/api/internal/oas"
)

//go:embed code.js
var code string

func sdkFor(docs oas.Document) ([]byte, error) {
	type node struct {
		path string
		base string
		elem map[string]*node
		item oas.PathItem
	}
	newNode := func(base, path string) *node {
		return &node{
			base: base,
			path: path,
			elem: make(map[string]*node),
		}
	}
	addItem := func(n *node, path string, item oas.PathItem) {
		elems := strings.Split(path, "/")
		for i := range elems {
			key := elems[i]
			if key == "" {
				continue
			}
			if _, ok := n.elem[key]; !ok {
				n.elem[key] = newNode("/"+key, strings.Join(elems[:i+1], "/"))
			}
			n = n.elem[key]
		}
		existing := &n.item
		if existing.Get == nil {
			existing.Get = item.Get
		}
		if existing.Post == nil {
			existing.Post = item.Post
		}
		if existing.Put == nil {
			existing.Put = item.Put
		}
		if existing.Delete == nil {
			existing.Delete = item.Delete
		}
		if existing.Patch == nil {
			existing.Patch = item.Patch
		}
		if existing.Options == nil {
			existing.Options = item.Options
		}
		if existing.Query == nil {
			existing.Query = item.Query
		}
	}

	var buf bytes.Buffer
	fmt.Fprintln(&buf, code)
	fmt.Fprintln(&buf, `/**`)
	fmt.Fprintf(&buf, ` * Returns a new API client for the %s API`+"\n", strings.TrimSuffix(string(docs.Information.Title), " API"))
	fmt.Fprintln(&buf, ` *`)
	fmt.Fprintln(&buf, ` * @param {string} host url for the API`)
	fmt.Fprintln(&buf, ` * @param {function} fetch function defaults to window.fetch`)
	fmt.Fprintln(&buf, ` * @returns {Object} API client`)
	fmt.Fprintln(&buf, ` */`)
	fmt.Fprintln(&buf, "export function API(host, fetch = globalThis.fetch) {")
	var paths []string
	for path := range docs.Paths {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var tree = newNode("/", "/")
	for i := range paths {
		addItem(tree, paths[i], docs.Paths[paths[i]])
	}
	// Each level of path value is a function of its own, whose variables must
	// not shadow those of the level around it while they are being set from them.
	var level func(n *node, nesting int)
	var resource func(n *node, object, url string, nesting int)
	line := func(nesting int, format string, args ...any) {
		fmt.Fprint(&buf, strings.Repeat("\t", nesting+1))
		fmt.Fprintf(&buf, format, args...)
		fmt.Fprintln(&buf)
	}
	valued := func(n *node) bool {
		return slices.ContainsFunc(slices.Collect(maps.Keys(n.elem)), isPathValue)
	}
	// callable writes the function for a resource with path values, starting
	// after the "=" of what it is assigned to.
	callable := func(n *node, url string, nesting int) {
		depth := nesting + 1
		line(nesting+1, "let client%d = {};", depth)
		line(nesting+1, "let path%d = `%s/${encodeURIComponent(value)}`;", depth, url)
		for _, key := range slices.Sorted(maps.Keys(n.elem)) {
			if isPathValue(key) {
				level(n.elem[key], nesting+1)
			}
		}
		line(nesting+1, "return client%d;", depth)
	}
	level = func(n *node, nesting int) {
		resource(n, fmt.Sprintf("client%d", nesting), fmt.Sprintf("${path%d}", nesting), nesting)
	}
	resource = func(n *node, object, url string, nesting int) {
		if n.item.Get != nil {
			line(nesting, "define(%s, 'GET', function(query) {", object)
			line(nesting, "\treturn fetch(`%s`+search(query), {", url)
			line(nesting, "\t\theaders: {")
			line(nesting, "\t\t\t'Accept': 'application/json'")
			line(nesting, "\t\t}")
			line(nesting, "\t}).then(wrap);")
			line(nesting, "});")
		}
		type method struct {
			Name string
			Does *oas.Operation
		}
		var methods = []method{
			{Name: "POST", Does: n.item.Post},
			{Name: "PUT", Does: n.item.Put},
			{Name: "DELETE", Does: n.item.Delete},
			{Name: "PATCH", Does: n.item.Patch},
			{Name: "OPTIONS", Does: n.item.Options},
			{Name: "QUERY", Does: n.item.Query},
		}
		for _, m := range methods {
			if m.Does == nil {
				continue
			}
			line(nesting, "define(%s, '%s', function(body) {", object, m.Name)
			line(nesting, "\treturn fetch(`%s`, {", url)
			// fetch has no way to send a method of this name in every browser.
			line(nesting, "\t\tmethod: '%s',", strings.ReplaceAll(m.Name, "QUERY", "POST"))
			line(nesting, "\t\tbody: JSON.stringify(body),")
			line(nesting, "\t\theaders: {")
			if m.Name == "QUERY" {
				line(nesting, "\t\t\t'X-HTTP-Method-Override': 'QUERY',")
			}
			line(nesting, "\t\t\t'Accept': 'application/json',")
			line(nesting, "\t\t\t'Content-Type': 'application/json'")
			line(nesting, "\t\t}")
			line(nesting, "\t}).then(wrap);")
			line(nesting, "});")
		}
		for _, key := range slices.Sorted(maps.Keys(n.elem)) {
			if isPathValue(key) {
				continue
			}
			child := n.elem[key]
			childObject, childURL := fmt.Sprintf("%s[%q]", object, key), url+"/"+key
			switch {
			case valued(child):
				line(nesting, "define(%s, %q, function(value) {", object, key)
				callable(child, childURL, nesting)
				line(nesting, "});")
			default:
				line(nesting, "define(%s, %q, {});", object, key)
			}
			resource(child, childObject, childURL, nesting)
		}
	}
	line(0, "let path0 = host;")
	switch {
	case valued(tree):
		line(0, "let client0 = function(value) {")
		callable(tree, "${path0}", 0)
		line(0, "};")
	default:
		line(0, "let client0 = {};")
	}
	level(tree, 0)
	line(0, "return client0;")
	fmt.Fprintln(&buf, "}")
	return buf.Bytes(), nil
}

func isPathValue(segment string) bool { return strings.HasPrefix(segment, "{") }
