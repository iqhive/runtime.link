package rest

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"runtime.link/api/internal/http"
	"runtime.link/api/internal/rest/rtags"
	"runtime.link/std"
)

var debug = os.Getenv("DEBUG_REST") != "" || os.Getenv("DEBUG_API") != ""

// Marshaler can be used to override the default JSON encoding of return values.
// This allows a custom format to be returned by a function.
type marshaler interface {
	MarshalREST() ([]byte, error)
}

// Resource describes a REST resource.
type Resource struct {
	Name string

	// Operations that can be performed on
	// this resource, keyed by HTTP method.
	Operations map[http.Method]Operation
}

// Operation describes a REST operation.
type Operation struct {
	std.Function

	// Parameters that can be passed to this operation.
	Parameters []Parameter

	// Possible responses returned by the operation,
	// keyed by HTTP status code.
	Responses map[int]reflect.Type

	argumentsNeedsMapping bool
}

type ParameterLocation int

const (
	ParameterInVoid ParameterLocation = -1
	ParameterInBody ParameterLocation = 0
	ParameterInPath ParameterLocation = 1 << iota
	ParameterInQuery
)

// Parameter description of an argument passed
// to a REST operation.
type Parameter struct {
	Name string
	Type reflect.Type

	Tags reflect.StructTag

	// Locations where the parameter
	// can be found in the request.
	Location ParameterLocation

	// index is the indicies of the
	// parameter indexing into the
	// function argument.
	Index []int
}

// Specification describes a rest API specification.
type Specification struct {
	std.Structure

	Resources map[string]Resource `api:"-"`

	duplicates []error
}

func SpecificationOf(rest std.Structure) (Specification, error) {
	var spec Specification
	if err := spec.setSpecification(rest); err != nil {
		return Specification{}, err
	}
	return spec, nil
}

func (spec *Specification) setSpecification(to std.Structure) error {
	spec.Structure = to
	return spec.load(to)
}

func (spec *Specification) load(from std.Structure) error {
	for _, fn := range from.Functions {
		if err := spec.loadOperation(fn); err != nil {
			return err
		}
	}
	for _, section := range from.Namespace {
		if err := spec.load(section); err != nil {
			return err
		}
	}
	return nil
}

func (spec *Specification) makeResponses(fn std.Function) (map[int]reflect.Type, error) {
	var responses = make(map[int]reflect.Type)
	var (
		rules = rtags.ResultRulesOf(string(fn.Tags.Get("rest")))
	)
	if len(rules) == 0 && fn.Type.NumOut() == 1 {
		responses[200] = fn.Type.Out(0)
		return responses, nil
	}
	var (
		p = newParser(fn)
	)
	if len(rules) != fn.Type.NumOut() {
		return nil, fmt.Errorf("%s result rules must match the number of return values", p.debug())
	}
	var fields []reflect.StructField
	for i := 0; i < fn.Type.NumOut(); i++ {
		fields = append(fields, reflect.StructField{
			Name: strings.Title(rules[i]),
			Tag:  reflect.StructTag(`json:"` + rules[i] + `"`),
			Type: fn.Type.Out(i),
		})
	}
	responses[200] = reflect.StructOf(fields)
	return responses, nil
}

func (spec *Specification) loadOperation(fn std.Function) error {
	tag := string(fn.Tags.Get("rest"))
	if tag == "-" {
		return nil //skip
	}
	if tag == "" {
		return fmt.Errorf("add a 'rest' tag to %s", fn.Name)
	}
	var (
		method, path, query string
	)
	splits := strings.Split(tag, " ")
	if len(splits) < 2 {
		return fmt.Errorf("make sure the 'rest' tag for %s is in the form '[METHOD] [PATH] (ARGUMENT_RULES) RESULT_RULES'", fn.Name)
	}
	method = splits[0]
	if method == "" {
		return fmt.Errorf("provide a method in the 'rest' tag for %s, ie 'GET %s'", fn.Name, tag)
	}
	var (
		params = newParser(fn)
		args   []reflect.Type
	)
	for i := 0; i < fn.Type.NumIn(); i++ {
		arg := fn.Type.In(i)
		args = append(args, arg)
	}
	splits = strings.SplitN(splits[1], "?", 2)
	path = splits[0]
	if err := params.parsePath(path, args); err != nil {
		return err
	}
	path = strings.ReplaceAll(path, "=%v", "")
	if len(splits) > 1 {
		query = "?" + splits[1]
		if err := params.parseQuery(query, args); err != nil {
			return err
		}
	}
	if err := params.parseBody(rtags.ArgumentRulesOf(tag)); err != nil {
		return err
	}
	responses, err := spec.makeResponses(fn)
	if err != nil {
		return err
	}
	resource := spec.Resources[path]
	if resource.Operations == nil {
		resource.Operations = make(map[http.Method]Operation)
	}
	// If two names collide, this is probably a mistake and we want to return an error.
	if existing, ok := resource.Operations[http.Method(method)]; ok {
		spec.duplicates = append(spec.duplicates, fmt.Errorf("by deduplicating the duplicate endpoint '%s %s' (%s and %s)",
			method, path, strings.Join(append(existing.Path, existing.Name), "."), strings.Join(append(fn.Path, fn.Name), ".")))
	}
	var argumentsNeedsMapping = false
	var count int
	for _, param := range params.list {
		if param.Location == ParameterInBody {
			count++
			if count > 1 {
				argumentsNeedsMapping = true
				break
			}
		}
	}
	if len(rtags.ArgumentRulesOf(string(fn.Tags.Get("rest")))) > 0 {
		argumentsNeedsMapping = true
	}
	resource.Operations[http.Method(method)] = Operation{
		Function:   fn,
		Parameters: params.list,
		Responses:  responses,

		argumentsNeedsMapping: argumentsNeedsMapping,
	}
	if spec.Resources == nil {
		spec.Resources = make(map[string]Resource)
	}
	spec.Resources[path] = resource
	return nil
}

type parser struct {
	pos int

	list []Parameter

	fn std.Function
}

func newParser(fn std.Function) *parser {
	return &parser{
		list: make([]Parameter, fn.Type.NumIn()),
		fn:   fn,
	}
}

func (p *parser) debug() string {
	return strings.Join(append(p.fn.Path, p.fn.Name), ".")
}

func (p *parser) parseBody(rules []string) error {
	if len(rules) == 0 {
		for i, param := range p.list {
			if param.Location == ParameterInBody {
				p.list[i].Type = p.fn.Type.In(i)
				p.list[i].Index = []int{i}
			}
		}
		return nil
	}
	var rule int
	for i, param := range p.list {
		if param.Location == ParameterInBody {
			if rule >= len(rules) {
				return fmt.Errorf("not enough argument rules for %s", p.debug())
			}
			p.list[i].Name = rules[rule]
			p.list[i].Index = []int{i}
			p.list[i].Type = p.fn.Type.In(i)
			rule++
		}
	}
	return nil
}

func (p *parser) parseParam(param string, args []reflect.Type, location ParameterLocation) error {
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
						p.list = append(p.list, Parameter{
							Name:     name,
							Type:     field.Type,
							Tags:     field.Tag,
							Index:    append(index, field.Index...),
							Location: ParameterInQuery,
						})
						continue
					}
					_, ok := std.TypeOf(field.Type)
					if ok {
						p.list = append(p.list, Parameter{
							Name:     name,
							Type:     field.Type,
							Tags:     field.Tag,
							Index:    append(index, field.Index...),
							Location: ParameterInQuery,
						})
						continue
					}
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
			p.list[p.pos].Location = ParameterInVoid
			// walk through and destructure each field as a
			// query parameter.
			if err := destructure(p.pos); err != nil {
				return err
			}
			p.pos++

		} else if strings.HasSuffix(param, "{}") {
			for i, arg := range args {
				if arg.Name() == strings.TrimSuffix(param, "{}") {
					p.list[i].Location = ParameterInVoid
					// walk through and destructure each field as a
					// query parameter.
					if err := destructure(i); err != nil {
						return err
					}
					break
				}
			}
		} else {
			if err := p.parseParam(param, args, ParameterInQuery); err != nil {
				return err
			}
		}
	}
	return nil
}

// parseStructParam parses a parameter that is located within a struct.
func (p *parser) parseStructParam(param string, args []reflect.Type) (Parameter, error) {
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
			return Parameter{
				Name:  param,
				Type:  arg,
				Index: append([]int{i}, index...),
			}, nil
		}
		// check if there are any matching struct tags.
		for i := 0; i < arg.NumField(); i++ {
			field := arg.Field(i)
			if name := field.Tag.Get("rest"); name == param {
				return Parameter{
					Name:  name,
					Type:  field.Type,
					Tags:  field.Tag,
					Index: append([]int{i}, field.Index...),
				}, nil
			}
		}
	}
	return Parameter{},
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
			if err := p.parseParam(param, args, ParameterInPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func toPtr(value *reflect.Value) interface{} {
	var ptr reflect.Value

	T := value.Type()

	if T.Kind() == reflect.Ptr {
		*value = reflect.New(T.Elem())
		ptr = *value
	} else {
		ptr = reflect.New(T)
		*value = ptr.Elem()
	}
	return ptr.Interface()
}
