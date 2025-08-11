// Package parser converts a go AST into a Go Source structure suitable for compilation.
package parser

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
	"runtime.link/xyz"
	"runtime.link/zgo/internal/source"
)

func Load(dir string, test bool) (map[string]source.Package, error) {
	config := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,

		Tests: true,
	}
	packages, err := packages.Load(config, dir)
	if err != nil {
		return nil, err
	}
	var results = make(map[string]source.Package)
	for _, pkg := range packages {
		loadPackage(config, results, pkg, test)
	}
	return results, nil
}

func locationIn(pkg *source.Package, node ast.Node, pos token.Pos) source.Location {
	return source.Location{
		Node:    node,
		FileSet: pkg.FileSet,
		Open:    pos,
		Shut:    pos,
	}
}

func locationRangeIn(pkg *source.Package, node ast.Node, pos, end token.Pos) source.Location {
	return source.Location{
		FileSet: pkg.FileSet,
		Node:    node,
		Open:    pos,
		Shut:    end,
	}
}

func typedIn(pkg *source.Package, node ast.Expr) source.Typed {
	return source.Typed{TV: pkg.Types[node], PKG: pkg.Name}
}

func loadSelection(pkg *source.Package, in *ast.SelectorExpr) source.Selection {
	sel := source.Selection{
		Location:  locationRangeIn(pkg, in, in.Pos(), in.End()),
		Typed:     typedIn(pkg, in),
		X:         loadExpression(pkg, in.X),
		Selection: loadExpression(pkg, in.Sel),
	}
	meta, ok := pkg.Selections[in]
	if ok && len(meta.Index()) > 1 && meta.Kind() == types.FieldVal {
		ptype := sel.X.TypeAndValue().Type.Underlying()
		for index := range meta.Index()[1:] {
			for {
				ptr, ok := ptype.(*types.Pointer)
				if !ok {
					break
				}
				ptype = ptr.Elem().Underlying()
			}
			rtype := ptype.(*types.Struct)
			field := rtype.Field(index)
			sel.Path = append(sel.Path, field.Name())
			ptype = field.Type().Underlying()
		}
	}
	return sel
}

func loadStar(pkg *source.Package, in *ast.StarExpr) source.Star {
	return source.Star{
		Location: locationRangeIn(pkg, in, in.Pos(), in.End()),
		Typed:    typedIn(pkg, in),
		WithLocation: source.WithLocation[source.Expression]{
			Value:          loadExpression(pkg, in.X),
			SourceLocation: locationIn(pkg, in, in.Star),
		},
	}
}

func loadComment(pkg *source.Package, in *ast.Comment) source.Comment {
	return source.Comment{
		Location: locationRangeIn(pkg, in, in.Pos(), in.End()),
		Slash:    locationIn(pkg, in, in.Slash),
		Text:     in.Text,
	}
}

func loadCommentGroup(pkg *source.Package, in *ast.CommentGroup) source.CommentGroup {
	var out source.CommentGroup
	out.Location = locationRangeIn(pkg, in, in.Pos(), in.End())
	for _, comment := range in.List {
		out.List = append(out.List, loadComment(pkg, comment))
	}
	return out
}

func loadField(pkg *source.Package, in *ast.Field) source.Field {
	var out source.Field
	out.Location = locationIn(pkg, in, in.Pos())
	if in.Doc != nil {
		out.Documentation = xyz.New(loadCommentGroup(pkg, in.Doc))
	}
	if in.Names != nil {
		var names []source.DefinedVariable
		for _, name := range in.Names {
			names = append(names, source.DefinedVariable(loadIdentifier(pkg, name)))
		}
		out.Names = xyz.New(names)
	}
	out.Type = loadType(pkg, in.Type)
	if in.Tag != nil {
		out.Tag = xyz.New(loadConstant(pkg, in.Tag))
	}
	if in.Comment != nil {
		out.Comment = xyz.New(loadCommentGroup(pkg, in.Comment))
	}
	return out
}

func loadFieldList(pkg *source.Package, in *ast.FieldList) source.FieldList {
	var out source.FieldList
	out.Location = locationRangeIn(pkg, in, in.Pos(), in.End())
	if in != nil {
		out.Opening = locationIn(pkg, in, in.Opening)
		for _, field := range in.List {
			out.Fields = append(out.Fields, loadField(pkg, field))
		}
		out.Closing = locationIn(pkg, in, in.Closing)
	}
	return out
}

func loadFile(pkg *source.Package, src *ast.File) source.File {
	var file source.File
	file.MinimumGoVersion = src.GoVersion
	if src.Doc != nil {
		file.Documentation = xyz.New(loadCommentGroup(pkg, src.Doc))
	}
	file.PackageKeyword = locationIn(pkg, src, src.Package)
	file.PackageName = source.ImportedPackage(loadIdentifier(pkg, src.Name))
	file.Location = locationRangeIn(pkg, src, src.FileStart, src.FileEnd)
	for _, comment := range src.Comments {
		file.Comments = append(file.Comments, loadCommentGroup(pkg, comment))
	}
	for _, imp := range src.Imports {
		file.Imports = append(file.Imports, loadImport(pkg, imp))
	}
	for _, bad := range src.Unresolved {
		file.Unresolved = append(file.Unresolved, loadIdentifier(pkg, bad))
	}
	for _, decl := range src.Decls {
		file.Definitions = append(file.Definitions, loadDefinitions(pkg, decl, true)...)
	}
	return file
}

func loadIdentifier(pkg *source.Package, in *ast.Ident) source.Identifier {
	var shadow int = -1
	var object types.Object
	if obj := pkg.Uses[in]; obj != nil {
		object = obj
	}
	if obj := pkg.Defs[in]; obj != nil {
		object = obj
	}
	var global bool
	if object != nil {
		for parent := object.Parent(); parent != nil; parent = parent.Parent() {
			if parent.Lookup(in.Name) != nil {
				shadow++
			}
		}
		parent := object.Parent()
		if parent != nil {
			global = parent.Parent() == types.Universe
		}
	}
	var isMethod = false
	function, ok := object.(*types.Func)
	if ok {
		if function.Type().(*types.Signature).Recv() != nil {
			isMethod = true
		}
	}
	return source.Identifier{
		Typed:    typedIn(pkg, in),
		Location: locationIn(pkg, in, in.Pos()),
		String:   in.Name,
		Method:   isMethod,
		Shadow:   shadow,
		Package:  global,
		Mutable:  true,
	}
}

func loadPackage(config *packages.Config, into map[string]source.Package, pkg *packages.Package, test bool) error {
	var loaded = source.Package{
		Info:    *pkg.TypesInfo,
		Name:    pkg.Name,
		FileSet: pkg.Fset,
		Test:    test,
	}
	if strings.HasSuffix(pkg.ID, ".test") {
		return nil
	}
	if (strings.HasSuffix(pkg.ID, ".test]") && !test) || (!strings.HasSuffix(pkg.ID, ".test]") && test) {
		return nil
	}
	for _, file := range pkg.Syntax {
		loaded.Files = append(loaded.Files, loadFile(&loaded, file))
	}
	into[pkg.Name] = loaded
	for _, imp := range pkg.Imports {
		if strings.HasPrefix(imp.Name, "internal/") {
			continue
		}
		switch imp.Name {
		case "reflect", "testing", "runtime", "os", "syscall", "unsafe", "math":
			continue
		}
		if _, ok := into[imp.Name]; !ok {
			into[imp.Name] = loaded

			packages, err := packages.Load(config, imp.Name)
			if err != nil {
				return err
			}
			for _, pkg := range packages {
				if err := loadPackage(config, into, pkg, false); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
