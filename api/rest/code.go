package rest

import (
	"bytes"
	_ "embed"
	"fmt"
	"sort"
	"strings"

	"runtime.link/api/internal/oas"
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
			if i == 0 {
				continue
			}
			key := elems[i]
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
	}

	var buf bytes.Buffer
	fmt.Fprintln(&buf, code)
	fmt.Fprintln(&buf, `/**`)
	fmt.Fprintf(&buf, ` * Returns a new API client for the %s API`+"\n", docs.Information.Title)
	fmt.Fprintln(&buf, ` *`)
	fmt.Fprintln(&buf, ` * @param {string} host url for the API`)
	fmt.Fprintln(&buf, ` * @param {function} fetch function defaults to window.fetch`)
	fmt.Fprintln(&buf, ` * @returns {Object} API client`)
	fmt.Fprintln(&buf, ` */`)
	fmt.Fprintln(&buf, "export function API(host, fetch) {")
	fmt.Fprintln(&buf, "\tlet path = host;")
	fmt.Fprintln(&buf, "\tlet client = {};")
	var paths []string
	for path := range docs.Paths {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var tree = newNode("/", "/")
	for i := range paths {
		addItem(tree, paths[i], docs.Paths[paths[i]])
	}
	var walk func(n *node, nesting int)
	walk = func(n *node, nesting int) {
		tabs := func() {
			for range nesting + 1 {
				fmt.Fprint(&buf, "\t")
			}
		}
		var path = n.path
		if nesting > 0 {
			path = n.base
			if strings.HasPrefix(path, "/{") {
				path = ""
			}
		}
		converted := strings.Join(strings.Split(path, "/"), ".")
		if path != "/" && path != "" {
			var variable bool
			for sub := range n.elem {
				if strings.HasPrefix(sub, "{") {
					variable = true
				}
			}
			if variable {
				tabs()
				fmt.Fprintf(&buf, "client%s = function(value) {\n", converted)
				tabs()
				fmt.Fprintf(&buf, "\tlet client = {};\n")
				tabs()
				fmt.Fprintf(&buf, "\tlet path = `${path}%s/${value}`;\n", path)
				for key, v := range n.elem {
					if strings.HasPrefix(key, "{") {
						walk(v, nesting+1)
					}
				}
				tabs()
				fmt.Fprintln(&buf, "\treturn client;")
				tabs()
				fmt.Fprintln(&buf, "}")
			} else {
				tabs()
				fmt.Fprintf(&buf, "client%s = {};\n", converted)
			}
		} else {
			converted = ""
		}
		if n.item.Get != nil {
			tabs()
			fmt.Fprintf(&buf, "client%s.GET = function(query) {\n", converted)
			tabs()
			fmt.Fprintf(&buf, "\treturn fetch(`${path}%s?`+new URLSearchParams(object).toString())", path)
			fmt.Fprintf(&buf, ".then(wrap);\n")
			tabs()
			fmt.Fprintf(&buf, "};\n")
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
		}
		for _, m := range methods {
			method := m.Name
			if m.Does == nil {
				continue
			}
			tabs()
			fmt.Fprintf(&buf, "client%s.%s = function(body) {\n", converted, method)
			tabs()
			fmt.Fprintf(&buf, "\treturn fetch(`${path}%s`, {\n", path)
			tabs()
			fmt.Fprintf(&buf, "\t\tmethod: '%s',\n", method)
			tabs()
			fmt.Fprintf(&buf, "\t\tbody: JSON.stringify(body),\n")
			tabs()
			fmt.Fprintf(&buf, "\t\theaders: {\n")
			tabs()
			fmt.Fprintf(&buf, "\t\t\t'Content-Type': 'application/json'\n")
			tabs()
			fmt.Fprintf(&buf, "\t\t}\n")
			tabs()
			fmt.Fprintf(&buf, "\t}).then(wrap);\n")
			tabs()
			fmt.Fprintf(&buf, "};\n")
		}
		for key, v := range n.elem {
			if strings.HasPrefix(key, "{") {
				continue
			}
			walk(v, nesting)
		}
	}
	walk(tree, 0)
	fmt.Fprint(&buf, "\t")
	fmt.Fprintln(&buf, "return client;")
	fmt.Fprintln(&buf, "}")
	return buf.Bytes(), nil
}
