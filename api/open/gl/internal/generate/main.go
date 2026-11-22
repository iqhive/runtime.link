package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/iqhive/runtime.link/api/xray"
)

type Registry struct {
	Comment    string      `xml:"comment"`
	Types      []Type      `xml:"types>type"`
	Enums      []Enums     `xml:"enums"`
	Groups     []Group     `xml:"groups>group"`
	Commands   []Command   `xml:"commands>command"`
	Features   []Feature   `xml:"feature"`
	Extensions []Extension `xml:"extensions>extension"`
}

type Type struct {
	Requires string `xml:"requires,attr"`
	Name     string `xml:"name,attr"`
	API      string `xml:"api,attr"`
	Comment  string `xml:"comment"`
	C        string `xml:",chardata"`
}

type Group struct {
	Name  string      `xml:"name,attr"`
	Enums []NameValue `xml:"enum"`
}

type NameValue struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type Enums struct {
	Namespace string `xml:"namespace,attr"`
	Type      string `xml:"type,attr"`
	Start     string `xml:"start,attr"`
	End       string `xml:"end,attr"`
	Vendor    string `xml:"vendor,attr"`
	Comment   string `xml:"comment"`
	Enums     []Enum `xml:"enum"`
}

type Enum struct {
	Value string `xml:"value,attr"`
	Name  string `xml:"name,attr"`
	API   string `xml:"api,attr"`
	Type  string `xml:"type,attr"`
	Group string `xml:"group,attr"`
	Alias string `xml:"alias,attr"`
}

type Command struct {
	Comment  string    `xml:"comment,attr"`
	Proto    Proto     `xml:"proto"`
	Param    []Param   `xml:"param"`
	Alias    NameValue `xml:"alias"`
	Vecequiv NameValue `xml:"vecequiv"`
	GLX      string    `xml:"glx,attr"`
}

type Proto struct {
	Group string `xml:"group,attr"`
	Name  string `xml:"name"`
	Ptype string `xml:"ptype"`
	C     string `xml:",chardata"`
}

type Param struct {
	Group string `xml:"group,attr"`
	Len   string `xml:"len,attr"`
	Class string `xml:"class,attr"`
	Ptype string `xml:"ptype"`
	Name  string `xml:"name"`
	C     string `xml:",chardata"`
}

type Extension struct {
	Name         string        `xml:"name,attr"`
	Supported    string        `xml:"supported,attr"`
	Protect      string        `xml:"protect,attr"`
	Requirements []Requirement `xml:"require"`
}

type Requirement struct {
	Comment  string      `xml:"comment,attr"`
	API      string      `xml:"api,attr"`
	Profile  string      `xml:"profile,attr"`
	Enums    []NameValue `xml:"enum"`
	Commands []NameValue `xml:"command"`
}

type Feature struct {
	API          string        `xml:"api,attr"`
	Name         string        `xml:"name,attr"`
	Protect      string        `xml:"protect,attr"`
	Number       string        `xml:"number,attr"`
	Comment      string        `xml:"comment"`
	Requirements []Requirement `xml:"require"`
}

// debugProc is the type of a GLDEBUGPROC in a call tag, which is what the Go
// type DebugFunc has to match for GL to be able to call it. GL keeps the
// function for as long as the context lasts, which is what the '~' says.
const debugProc = "~func(unsigned_int,unsigned_int,unsigned_int,unsigned_int,int,&#char,&#void)"

// valueTypes returns the Go type that a GL type is declared as when it is
// passed by value, and the C type that api/call is told it is. Both are
// empty for a GL type that is not known.
func valueTypes(param Param) (goType, cType string) {
	switch param.Ptype {
	case "GLenum":
		if param.Group != "" {
			return param.Group, "unsigned_int"
		}
		if param.Name == "binaryFormat" {
			return "BinaryFormat", "unsigned_int"
		}
		return "Enum", "unsigned_int"
	case "GLbitfield":
		if param.Group != "" {
			return param.Group, "unsigned_int"
		}
		return "uint32", "unsigned_int"
	case "GLuint":
		return "uint32", "unsigned_int"
	case "GLint":
		return "int32", "int"
	case "GLsizei":
		return "int", "int"
	case "GLfloat":
		return "float32", "float"
	case "GLdouble":
		return "float64", "double"
	case "GLboolean":
		// an unsigned char that is 0 or 1, which C passes and returns as it does a bool.
		return "bool", "bool"
	case "GLubyte":
		return "uint8", "unsigned_char"
	case "GLbyte":
		return "int8", "signed_char"
	case "GLushort":
		return "uint16", "unsigned_short"
	case "GLshort":
		return "int16", "short"
	case "GLsizeiptr", "GLintptr":
		return "uintptr", "ptrdiff_t"
	case "GLchar":
		return "byte", "char"
	case "GLuint64":
		return "uint64", "uint64_t"
	case "GLint64":
		return "int64", "int64_t"
	}
	// a GL type that is not known here has no types, which stops the generator.
	return "", ""
}

// types returns how a parameter, or the result of a command, is declared in Go
// and in the call tag of its function. Whether it is a pointer is in the C
// text around its type, where each '*' is one more level of it: a 'const T *'
// is as much of a pointer as a 'T *' is.
func types(param Param, result bool) (goType, cType string) {
	depth := strings.Count(param.C, "*")
	borrowed := "&"
	if strings.Contains(param.C, "const") {
		borrowed = "&#"
	}
	switch {
	case param.Ptype == "GLDEBUGPROC":
		return "DebugFunc", debugProc
	case param.Ptype == "GLsync" && depth == 0:
		// a pointer to a struct that only GL knows the fields of.
		return "uintptr", "&void"
	case depth == 0:
		return valueTypes(param)
	case param.Ptype == "":
		return strings.Repeat("*", depth-1) + "Pointer", borrowed + "void"
	case depth == 1 && borrowed == "&#" && (param.Ptype == "GLchar" || (result && param.Ptype == "GLubyte")):
		// text that ends in a NUL, which is the string that glGetString returns as well.
		return "string", "&#char"
	}
	goType, cType = valueTypes(param)
	if param.Ptype == "GLsizei" {
		// a value is converted to the int that C takes, and what a pointer
		// points to is not: it has to be as wide as what C reads and writes.
		goType = "int32"
	}
	if depth > 1 {
		cType = "void"
	}
	return strings.Repeat("*", depth) + goType, borrowed + cType
}

func noReservedWords(s string) string {
	// OpenGL has some reserved words that are not valid in Go.
	switch s {
	case "const", "type", "func", "interface", "struct", "map", "chan", "package", "range":
		return s + "_"
	default:
		return s
	}
}

func main() {
	var local io.Reader
	local, err := os.Open("gl.xml")
	if err != nil && !os.IsNotExist(err) {
		log.Fatal(xray.New(err))
	}
	if err != nil {
		resp, err := http.Get("https://raw.githubusercontent.com/KhronosGroup/OpenGL-Registry/refs/heads/main/xml/gl.xml")
		if err != nil {
			log.Fatal(xray.New(err))
		}
		defer resp.Body.Close()
		write, err := os.Create("gl.xml")
		if err != nil {
			log.Fatal(xray.New(err))
		}
		local = io.TeeReader(resp.Body, write)
	}
	var registry Registry
	if err := xml.NewDecoder(local).Decode(&registry); err != nil {
		log.Fatal(xray.New(err))
	}
	var commands = make(map[string]Command)
	for _, command := range registry.Commands {
		commands[command.Proto.Name] = command
	}
	var v [4]*os.File
	for i := range v {
		v[i], err = os.Create("v" + fmt.Sprint(i+1) + ".go")
		if err != nil {
			log.Fatal(xray.New(err))
		}
		defer v[i].Close()
		fmt.Fprintf(v[i], "// Code generated by generate.go; DO NOT EDIT.\n")
		fmt.Fprintf(v[i], "package gl\n\ntype V%v struct{", i+1)
	}
	var groups_used = make(map[string]bool)
	var done = make(map[string]bool)
	for _, feature := range registry.Features {
		if !strings.HasPrefix(feature.Name, "GL_VERSION_ES") && !strings.HasPrefix(feature.Name, "GL_ES_VERSION_") && !strings.HasPrefix(feature.Name, "GL_SC_VERSION_") {
			fmt.Println(feature.Name)
			var i = 0
			fmt.Sscan(string(feature.Number[0]), &i)
			i--
			for _, req := range feature.Requirements {

				for _, cmd := range req.Commands {
					if done[cmd.Name] {
						continue
					}
					command, ok := commands[cmd.Name]
					if !ok {
						log.Fatalf("command %s not found in gl.xml", cmd.Name)
					}
					// the tag is what api/call links the function by: the symbol, and the C type of it.
					var tag strings.Builder
					fmt.Fprintf(&tag, "%s func(", command.Proto.Name)
					fmt.Fprintf(v[i], "\n\t%s func(", strings.TrimPrefix(command.Proto.Name, "gl"))
					if len(command.Param) > 0 {
						for j, param := range command.Param {
							if j > 0 {
								fmt.Fprintf(v[i], ", ")
								tag.WriteString(",")
							}
							if param.Group != "" {
								groups_used[param.Group] = true
							}
							goType, cType := types(param, false)
							if cType == "" {
								log.Fatalf("%s: no types for the parameter %s, a %q", cmd.Name, param.Name, param.Ptype)
							}
							fmt.Fprintf(v[i], "%s %s", noReservedWords(param.Name), goType)
							tag.WriteString(cType)
						}
					}
					fmt.Fprintf(v[i], ") ")
					tag.WriteString(")")
					if command.Proto.Group != "" {
						groups_used[command.Proto.Group] = true
					}
					// a command that returns nothing is declared as 'void', which is no type that gl.xml marks up.
					if command.Proto.Ptype != "" || strings.Contains(command.Proto.C, "*") {
						goType, cType := types(Param{Ptype: command.Proto.Ptype, C: command.Proto.C, Group: command.Proto.Group}, true)
						if cType == "" {
							log.Fatalf("%s: no types for the result, a %q", cmd.Name, command.Proto.Ptype)
						}
						fmt.Fprint(v[i], goType)
						tag.WriteString(cType)
					}
					fmt.Fprintf(v[i], " `call:\"%v\"`", tag.String())
					done[cmd.Name] = true
				}
			}
		}
	}
	for i := range v {
		fmt.Fprintf(v[i], "\n}\n\n")
	}
	var groups = make(map[string][]Enum)
	var reused = make(map[string]bool)
	for _, enums := range registry.Enums {
		for _, enum := range enums.Enums {
			if enum.Group != "" {
				var count int
				for group := range strings.SplitSeq(enum.Group, ",") {
					if groups_used[group] {
						groups[group] = append(groups[group], enum)
					}
					count++
				}
				if count > 1 {
					reused[enum.Name] = true // this enum is reused in multiple groups
				}
			}
		}
	}
	enumsGo, err := os.Create("enums.go")
	if err != nil {
		log.Fatal(xray.New(err))
	}
	defer enumsGo.Close()
	fmt.Fprintf(enumsGo, "// Code generated by generate.go; DO NOT EDIT.\n")
	fmt.Fprintf(enumsGo, "package gl\n\n")
	for name, group := range groups {
		if len(group) == 0 {
			continue
		}
		var prefix = name
		if reused[name] {
			prefix = name
		}
		fmt.Fprintf(enumsGo, "type %s uint32\n\n", name)
		fmt.Fprintf(enumsGo, "const (\n")
		for _, enum := range group {
			if enum.API != "" {
				continue // skip API specific enums
			}
			fmt.Fprintf(enumsGo, "\t%s %s = %s\n", prefix+noReservedWords(enum.Name), name, enum.Value)
		}
		fmt.Fprintf(enumsGo, ")\n\n")
	}
}
