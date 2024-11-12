package zigc

import (
	"fmt"
	"io"
	"path"
	"reflect"
	"strconv"
	"strings"

	"runtime.link/zgo/internal/source"
)

type Target struct {
	io.Writer

	Tabs int

	CurrentPackage string
}

func (zig Target) Compile(node source.Node) error {
	rtype := reflect.TypeOf(node)
	method := reflect.ValueOf(&zig).MethodByName(rtype.Name())
	if !method.IsValid() {
		return fmt.Errorf("unsupported node type: %s", rtype.Name())
	}
	err := method.Call([]reflect.Value{reflect.ValueOf(node)})
	if len(err) > 0 && !err[0].IsNil() {
		return err[0].Interface().(error)
	}
	return nil
}

func (zig Target) toString(node source.Node) string {
	var buf strings.Builder
	zig.Writer = &buf
	zig.Tabs = 0
	zig.Compile(node)
	return buf.String()
}

func (zig Target) Selection(sel source.Selection) error {
	if err := zig.Compile(sel.X); err != nil {
		return err
	}
	for _, elem := range sel.Path {
		fmt.Fprintf(zig, ".%s", elem)
	}
	fmt.Fprintf(zig, ".")
	return zig.Compile(sel.Selection)
}

func (zig Target) Star(star source.Star) error {
	if err := zig.Compile(star.Value); err != nil {
		return err
	}
	fmt.Fprintf(zig, ".get()")
	return nil
}

func (zig *Target) File(file source.File) error {
	for _, decl := range file.Declarations {
		if err := zig.Compile(decl); err != nil {
			return err
		}
	}
	return nil
}

func (zig *Target) Identifier(id source.Identifier) error {
	if id.IsPackage {
		fmt.Fprintf(zig, "%s", zig.PackageOf(id.String))
		return nil
	}
	if id.String == "_" {
		_, err := zig.Write([]byte("_"))
		return err
	}
	if id.Shadow > 0 {
		fmt.Fprintf(zig, `@"%s.%d"`, id.String, id.Shadow)
		return nil
	}
	_, err := zig.Write([]byte(id.String))
	return err
}

func (zig *Target) Package(pkg *source.Package) error {
	fmt.Fprintf(zig, "const go = @import(\"go.zig\");\n")
	var imports = make(map[string]bool)
	for _, f := range pkg.Files {
		for _, imp := range f.Imports {
			name, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return fmt.Errorf("invalid import path: %v", err)
			}
			imports[name] = true
		}
	}
	delete(imports, "testing")
	delete(imports, "math")
	for name := range imports {
		fmt.Println(name)
		fmt.Fprintf(zig, `const %s = @import("%s.zig");`+"\n", path.Base(name), name)
	}
	for _, f := range pkg.Files {
		if err := zig.File(f); err != nil {
			return err
		}
	}
	return nil
}

func (zig Target) PackageOf(name string) string {
	if name == "testing" {
		return "go.testing"
	}
	if name == "math" {
		return "go.math"
	}
	return name
}

func (zig Target) Declaration(decl source.Declaration) error {
	node, _ := decl.Get()
	return zig.Compile(node)
}

func (zig Target) DeclarationGroup(decl source.DeclarationGroup) error {
	for _, spec := range decl.Specifications {
		if err := zig.Compile(spec); err != nil {
			return err
		}
	}
	return nil
}
