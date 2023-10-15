package api

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type parameterLocation int

const (
	parameterInVoid parameterLocation = -1
	parameterInBody parameterLocation = 0
	parameterInPath parameterLocation = 1 << iota
	parameterInQuery
)

// parameter description of an argument passed
// to a REST operation.
type parameter struct {
	Name string
	Type reflect.Type

	Tags reflect.StructTag

	// Locations where the parameter
	// can be found in the request.
	Location parameterLocation

	// index is the indicies of the
	// parameter indexing into the
	// function argument.
	Index []int
}

type parser struct {
	pos int

	list []parameter

	fn Function
}

func newParser(fn Function) *parser {
	return &parser{
		list: make([]parameter, fn.NumIn()),
		fn:   fn,
	}
}

func (p *parser) debug() string {
	return strings.Join(append(p.fn.Path, p.fn.Name), ".")
}

func (p *parser) parseBody(rules []string) error {
	if len(rules) == 0 {
		for i, param := range p.list {
			if param.Location == parameterInBody {
				p.list[i].Type = p.fn.In(i)
				p.list[i].Index = []int{i}
			}
		}
		return nil
	}
	var rule int
	for i, param := range p.list {
		if param.Location == parameterInBody {
			if rule >= len(rules) {
				return fmt.Errorf("not enough argument rules for %s", p.debug())
			}
			p.list[i].Name = rules[rule]
			p.list[i].Index = []int{i}
			p.list[i].Type = p.fn.In(i)
			rule++
		}
	}
	return nil
}

func (p *parser) parseParam(param string, args []reflect.Type, location parameterLocation) error {
	if strings.Contains(param, "=") || strings.HasPrefix(param, "%") {
		name, format, ok := strings.Cut(param, "=")
		if !ok {
			format = name
		}
		if format[0] != '%' {
			return nil //FIXME need to do something to support constants?
		}
		if format[len(format)-1] != 'v' {
			return fmt.Errorf("%s format parameter must end with v", p.debug())
		}
		if format[1] == '[' {
			//extract the numerial between the brackets.
			if !strings.Contains(format[2:len(format)-1], "]") {
				return fmt.Errorf("%s format parameter with numeral must have closing bracket", p.debug())
			}

			numeral := strings.SplitN(format[2:len(format)-1], "]", 2)[0]
			i, err := strconv.Atoi(numeral)
			if err != nil {
				return fmt.Errorf("%s format parameter with numeral must be a number", p.debug())
			}
			p.pos = i
		} else {
			p.pos++
		}
		if p.pos-1 >= len(args) {
			return fmt.Errorf("%s format parameter %d is out of range (if you are referencing struct fields, omit the =%%v)", p.debug(), p.pos)
		}
		var (
			existing = &p.list[p.pos-1]
		)
		if existing.Name != "" && existing.Name != name {
			return fmt.Errorf("%s duplicate parameters must share the same name", p.debug())
		}
		if existing.Name == "" {
			existing.Name = name
		}
		existing.Type = args[p.pos-1]
		existing.Location |= location
		existing.Index = []int{p.pos - 1}
		return nil

	} else {
		result, err := p.parseStructParam(param, args)
		if err != nil {
			return err
		}
		result.Location |= location
		p.list = append(p.list, result)
		return nil
	}
}

func (p *parser) parseQuery(query string, args []reflect.Type) error {
	if query == "" {
		return nil
	}
	if query[0] != '?' {
		return fmt.Errorf("%s query must start with ?", p.debug())
	}
	params := strings.Split(query[1:], "&")
	for _, param := range params {

		// It's possible to destructure a Go struct into
		// a collection of possible query arguments. This
		// is represented with a '{}', ie. GET /path/to/endpoint?DoSomethingRequest{}
		// The name of this should map to the type in the function arguments.
		destructure := func(i int) error {
			if i >= len(args) {
				return fmt.Errorf("%s destructured parameter %d is out of range", p.debug(), i)
			}
			var (
				arg = args[i]
			)
			if arg.Kind() != reflect.Struct {
				return fmt.Errorf("%s only structs can be destructured, '%v' is not a struct", p.debug(), arg)
			}
			var walk func(arg reflect.Type, index []int, parent string)
			walk = func(arg reflect.Type, index []int, parent string) {
				for i := 0; i < arg.NumField(); i++ {
					var (
						field = arg.Field(i)
					)
					if !field.IsExported() {
						continue
					}
					name := field.Tag.Get("rest")
					if name == "" {
						name = field.Tag.Get("json")
					}
					if name == "" {
						name = field.Name
					}
					if parent != "" {
						name = parent + "." + name
					}
					if field.Type.Kind() != reflect.Struct {
						p.list = append(p.list, parameter{
							Name:     name,
							Type:     field.Type,
							Tags:     field.Tag,
							Index:    append(index, field.Index...),
							Location: parameterInQuery,
						})
						continue
					}
					/*_, ok := std.TypeOf(field.Type)
					if ok {
						p.list = append(p.list, Parameter{
							Name:     name,
							Type:     field.Type,
							Tags:     field.Tag,
							Index:    append(index, field.Index...),
							Location: ParameterInQuery,
						})
						continue
					}*/
					walk(field.Type, append(index, field.Index...), name)
				}
			}
			walk(arg, []int{i}, "")
			return nil
		}
		if strings.HasPrefix(param, "%") {
			if p.pos >= len(p.list) {
				return fmt.Errorf("%s query parameter %d is out of range", p.debug(), p.pos)
			}
			p.list[p.pos].Location = parameterInVoid
			// walk through and destructure each field as a
			// query parameter.
			if err := destructure(p.pos); err != nil {
				return err
			}
			p.pos++

		} else if strings.HasSuffix(param, "{}") {
			for i, arg := range args {
				if arg.Name() == strings.TrimSuffix(param, "{}") {
					p.list[i].Location = parameterInVoid
					// walk through and destructure each field as a
					// query parameter.
					if err := destructure(i); err != nil {
						return err
					}
					break
				}
			}
		} else {
			if err := p.parseParam(param, args, parameterInQuery); err != nil {
				return err
			}
		}
	}
	return nil
}

// parseStructParam parses a parameter that is located within a struct.
func (p *parser) parseStructParam(param string, args []reflect.Type) (parameter, error) {
	var (
		path = strings.Split(param, ".")
	)
	for i, arg := range args {
		for arg.Kind() == reflect.Ptr {
			arg = arg.Elem()
		}
		if arg.Kind() != reflect.Struct {
			continue
		}
		found := true // by exception
		index := []int{}
		for _, name := range path {
			field, ok := arg.FieldByName(name)
			if len(field.Index) > 1 {
				panic("my assumption was wrong")
			}
			if !ok {
				found = false
			} else {
				index = append(index, field.Index...)
				arg = field.Type
			}
		}
		if found {
			return parameter{
				Name:  param,
				Type:  arg,
				Index: append([]int{i}, index...),
			}, nil
		}
		// check if there are any matching struct tags.
		for i := 0; i < arg.NumField(); i++ {
			field := arg.Field(i)
			if name := field.Tag.Get("rest"); name == param {
				return parameter{
					Name:  name,
					Type:  field.Type,
					Tags:  field.Tag,
					Index: append([]int{i}, field.Index...),
				}, nil
			}
		}
	}
	return parameter{},
		fmt.Errorf(
			"%s parameter %s needs either a matching field "+
				"name or a matching 'rest' struct tag.",
			param, p.debug())
}

func (p *parser) parsePath(path string, args []reflect.Type) error {
	if path == "" {
		return nil
	}
	if path[0] != '/' {
		return fmt.Errorf("%s path must start with /", p.debug())
	}
	// each parameter in the path is contained within a pair of braces.
	if strings.Contains(path, "{") {
		var params []string
		for _, candidate := range strings.Split(path[1:], "{")[1:] {
			splits := strings.SplitN(candidate, "}", 2)
			if len(splits) != 2 {
				return fmt.Errorf("%s path parameter must be in the form {param}, with a closing brace", p.debug())
			}
			params = append(params, splits[0])
		}
		for _, param := range params {
			if err := p.parseParam(param, args, parameterInPath); err != nil {
				return err
			}
		}
	}
	return nil
}
