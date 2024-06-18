package source

import (
	"go/ast"
	"io"

	"runtime.link/xyz"
)

type File struct {
	MinimumGoVersion string

	Documentation xyz.Maybe[CommentGroup]
	Keyword       Location
	Name          Identifier
	Declarations  []Declaration
	FileFrom      Location
	FileUpto      Location
	Imports       []SpecificationImport
	Unresolved    []Identifier
	Comments      []CommentGroup
}

func (pkg *Package) loadFile(src *ast.File) File {
	var file File
	file.MinimumGoVersion = src.GoVersion
	if src.Doc != nil {
		file.Documentation = xyz.New(pkg.loadCommentGroup(src.Doc))
	}
	file.Keyword = pkg.location(src.Package)
	file.Name = pkg.loadIdentifier(src.Name)
	file.FileFrom = pkg.location(src.FileStart)
	file.FileUpto = pkg.location(src.FileEnd)
	for _, comment := range src.Comments {
		file.Comments = append(file.Comments, pkg.loadCommentGroup(comment))
	}
	for _, imp := range src.Imports {
		file.Imports = append(file.Imports, pkg.loadImport(imp))
	}
	for _, bad := range src.Unresolved {
		file.Unresolved = append(file.Unresolved, pkg.loadIdentifier(bad))
	}
	for _, decl := range src.Decls {
		file.Declarations = append(file.Declarations, pkg.loadDeclaration(decl))
	}
	return file
}

func (file *File) Compile(w io.Writer) error {
	for _, decl := range file.Declarations {
		if err := decl.compile(w, 0); err != nil {
			return err
		}
	}
	return nil
}
