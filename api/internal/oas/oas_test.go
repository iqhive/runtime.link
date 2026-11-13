package oas

import (
	goscanner "go/scanner"
	htmltemplate "html/template"
	"reflect"
	"slices"
	"testing"
	textscanner "text/scanner"
	texttemplate "text/template"
)

// TestDocumentKeepsSchemasByType checks that a document tells apart the types
// of two packages whose paths end alike: they were one schema when they had one
// name, as a schema was kept by the last element of its package path.
func TestDocumentKeepsSchemasByType(t *testing.T) {
	var (
		goScanner    = reflect.TypeFor[goscanner.Scanner]()
		textScanner  = reflect.TypeFor[textscanner.Scanner]()
		textPosition = reflect.TypeFor[textscanner.Position]()
		goError      = reflect.TypeFor[goscanner.Error]()
	)
	var doc Document
	if doc.Lookup(goScanner) != nil {
		t.Fatal("an empty document has a schema")
	}
	doc.Register(goScanner, "Scanner", &Schema{Title: "go"})
	if doc.Lookup(textScanner) != nil {
		t.Fatal("text/scanner.Scanner is found once go/scanner.Scanner is registered")
	}
	doc.Register(textScanner, "Scanner", &Schema{Title: "text"})
	doc.Register(textPosition, "Position", &Schema{Title: "position"})
	doc.Register(goError, "Error", &Schema{Title: "error"})

	for rtype, want := range map[reflect.Type]URI{
		goScanner:    "#/components/schemas/scanner/$defs/Scanner",
		goError:      "#/components/schemas/scanner/$defs/Error",
		textScanner:  "#/components/schemas/text.scanner/$defs/Scanner",
		textPosition: "#/components/schemas/text.scanner/$defs/Position",
	} {
		if got := doc.Lookup(rtype); got == nil || got.Ref != want {
			t.Errorf("%v is referred to as %+v, want %s", rtype, got, want)
		}
	}
	if got := doc.Components.Schemas["scanner"].Defs["Scanner"].Title; got != "go" {
		t.Errorf("scanner/Scanner is the schema %q", got)
	}
	if got := doc.Components.Schemas["text.scanner"].Defs["Scanner"].Title; got != "text" {
		t.Errorf("text.scanner/Scanner is the schema %q", got)
	}

	// A type that is registered again keeps its place.
	doc.Register(textScanner, "Scanner", &Schema{Title: "again"})
	if got := doc.Components.Schemas["text.scanner"].Defs["Scanner"].Title; got != "again" || len(doc.Components.Schemas) != 2 {
		t.Errorf("registered again, text/scanner.Scanner is %q among %d namespaces", got, len(doc.Components.Schemas))
	}

	// Two types of one package that go by one name are told apart as well,
	// as the instances of a generic type may be once their names are cut.
	doc.Register(reflect.TypeFor[goscanner.ErrorList](), "Error", &Schema{Title: "list"})
	if got := doc.Lookup(reflect.TypeFor[goscanner.ErrorList]()); got.Ref != "#/components/schemas/scanner/$defs/Error_2" {
		t.Errorf("a second type by the name of Error is referred to as %s", got.Ref)
	}
	if got := doc.Lookup(goError); got.Ref != "#/components/schemas/scanner/$defs/Error" {
		t.Errorf("the first type by the name of Error is now referred to as %s", got.Ref)
	}
}

// TestDocumentKeepsAPackageInOneNamespace checks that every type of a package
// is in the namespace its package was given, whether or not its own name is one
// that another package has.
func TestDocumentKeepsAPackageInOneNamespace(t *testing.T) {
	var doc Document
	doc.Register(reflect.TypeFor[texttemplate.Template](), "Template", &Schema{})
	doc.Register(reflect.TypeFor[htmltemplate.Template](), "Template", &Schema{})
	doc.Register(reflect.TypeFor[texttemplate.ExecError](), "ExecError", &Schema{})
	doc.Register(reflect.TypeFor[htmltemplate.Error](), "Error", &Schema{})
	var namespaces []string
	for namespace, schema := range doc.Components.Schemas {
		if len(schema.Defs) != 2 {
			t.Errorf("the namespace %s holds %d schemas, want the 2 of its package", namespace, len(schema.Defs))
		}
		namespaces = append(namespaces, namespace)
	}
	slices.Sort(namespaces)
	if !slices.Equal(namespaces, []string{"html.template", "template"}) {
		t.Errorf("the namespaces are %q", namespaces)
	}
}

// TestSchemaKeepsDefinitionsByType checks the same of the definitions inside
// one schema, which have no namespaces to be in.
func TestSchemaKeepsDefinitionsByType(t *testing.T) {
	var (
		goScanner   = reflect.TypeFor[goscanner.Scanner]()
		textScanner = reflect.TypeFor[textscanner.Scanner]()
	)
	var root Schema
	if root.Lookup(goScanner) != nil {
		t.Fatal("an empty schema has a definition")
	}
	root.Register(goScanner, "Scanner", &Schema{Type: []Type{Types.Object}, Title: "go"})
	if root.Lookup(textScanner) != nil {
		t.Fatal("text/scanner.Scanner is found once go/scanner.Scanner is registered")
	}
	root.Register(textScanner, "Scanner", &Schema{Type: []Type{Types.Object}, Title: "text"})
	root.Register(textScanner, "Scanner", &Schema{Type: []Type{Types.Object}, Title: "text again"})

	if got := root.Lookup(goScanner); got == nil || got.Ref != "#/$defs/Scanner" || !slices.Equal(got.Type, []Type{Types.Object}) {
		t.Errorf("go/scanner.Scanner is referred to as %+v", got)
	}
	if got := root.Lookup(textScanner); got == nil || got.Ref != "#/$defs/scanner.Scanner" {
		t.Errorf("text/scanner.Scanner is referred to as %+v", got)
	}
	if len(root.Defs) != 2 || root.Defs["Scanner"].Title != "go" || root.Defs["scanner.Scanner"].Title != "text again" {
		t.Errorf("the definitions are %+v", root.Defs)
	}
}

func TestNamesThatAreTaken(t *testing.T) {
	if got := qualifiers("github.com/a/b"); !slices.Equal(got, []string{"b", "a.b", "github.com.a.b"}) {
		t.Errorf("qualifiers = %q", got)
	}
	if got := qualifiers("main"); !slices.Equal(got, []string{"main"}) {
		t.Errorf("qualifiers = %q", got)
	}
	taken := map[string]bool{}
	for _, want := range []string{"b", "a.b", "b_2", "b_3"} {
		got := vacant([]string{"b", "a.b"}, func(name string) bool { return taken[name] })
		if got != want {
			t.Fatalf("with %v taken the name is %q, want %q", taken, got, want)
		}
		taken[got] = true
	}
}
