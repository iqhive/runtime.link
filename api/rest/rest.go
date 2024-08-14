/*
Package rest provides a REST API transport.

	var API struct {
		api.Specification `rest:"http://api.example.com/v1"`

		Echo func(message string) string `rest:"GET /echo?message=%v"`
	}

When Echo is called, it calls the function over HTTPS/REST to
'https://api.example.com/v1/echo' and returns the result (if there
is an error and the function doesn't return one, it will panic).

The REST transport will be served by the [api.Handler] when
the Content-Type of the request is 'application/json' and the
API has suitable rest tags.

	API.Echo = func(message string) string { return message }
	api.ListenAndServe(":"+os.Getenv("PORT"), &API)

This starts a local HTTP server and listens on PORT
for requests to /echo and responds to these requests with the
defined Echo function. Arguments and Results are automatically
converted to the Content-Type where possible.

You can return receive-only channels which will be served to
clients as a websocket.

# Tags

Each API function can have a rest tag that formats
function arguments (%v) to query parameters.
Each tag must follow the space-seperated pattern:

	GET /path/to/endpoint/{object=%v}?query=%v (argument,mapping,rules) result,mapping,rules
	[METHOD](CONTENT_TYPE) [PATH] (ARGUMENT_RULES) RESULT_RULES

It begins with a METHOD, followed by an optional body CONTENT_TYPE
then with a PATH format string that descibes how the function
arguments are mapped onto the HTTP path & query. This is an
extension of [http.ServeMux] with support for fmt rules and
content types.

The CONTENT_TYPE (when unspecified) defaults to:

  - text/plain for booleans, numerical values, time.Time, []byte and strings.
  - application/json for structs, arrays, slices and maps.

The path can contain path expansion parameters {name=%v} or
ordinary format parameters %v (similar to the fmt package).
Think of the arguments of the function as the parameters that
get passed to a printf call. Imagine it working like this:

	http.Get(fmt.Sprintf("/path/with/%v?query=%v", value, query))

If the query parameter is untitled, then the value will be
expanded as key-value query parameters. This applies to structs
and maps:

	GET /path/to/endpoint?%v

If a path or query expansion parameter omits a format parameter,
the value will be considered to be nested within a struct argument
and the name of the parameter will be used to look for the first
matching field in subsequent body structures. Either by field
name or by rest tag.

	POST /path/to/endpoint/{ID}
	{
		ID: "1234",
		Value: "something"
	}

ARGUMENT_RULES are optional, they are a comma separated list
of names to give the remaining arguments in the JSON body
of the request. By default, arguments are posted as an
array, however if there are ARGUMENT_RULES, the arguments
will be mapped into json fields of the name, matching the
argument's position.

	foo func(id int, value string) `rest:"POST /foo (id,value)"`
	foo(22, "Hello World") => {"id": 22, "value":"Hello World"}

RESULT_RULES are much like ARGUMENT_RULES, except they operate
on the results of the function instead of the arguments. They
map named json fields to the result values.

	getLatLong func() (float64, float64) `rest:"GET /latlong latitude,longitude"`
	{"latitude": 12.2, "longitude": 15.0} => lat, lon := getLatLong()

# Response Headers

In order to read and write HTTP headers in a request, values should implement the
following interfaces.

	type HeaderWriter interface {
		WriteHeadersHTTP(http.Header)
	}

	type HeaderReader interface {
		ReadHeadersHTTP(http.Header)
	}

If multiple result values implement these interfaces, they will be called in the order
they are returned. Here's an example:

	type ProfilePicture struct {
		io.ReadCloser
	}

	func (ProfilePicture) WriteHeadersHTTP(header http.Header) {
		header.Set("Content-Type", "image/png")
	}

	type API struct {
		api.Specification

		GetProfilePicture func() (ProfilePicture, error)
	}
*/
package rest

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"runtime.link/api"
	http_api "runtime.link/api/internal/http"
	"runtime.link/api/internal/oas"
	"runtime.link/api/internal/rtags"
	"runtime.link/api/xray"
)

var debug = os.Getenv("DEBUG_REST") != "" || os.Getenv("DEBUG_API") != ""


// Marshaler can be used to override the default JSON encoding of return values.
// This allows a custom format to be returned by a function.
type marshaler interface {
	MarshalREST() ([]byte, error)
}

// resource describes a REST resource.
type resource struct {
	Name string

	// Operations that can be performed on
	// this resource, keyed by HTTP method.
	Operations map[http_api.Method]operation
}

// operation describes a REST operation.
type operation struct {
	api.Function

	DefaultContentType oas.ContentType

	// Parameters that can be passed to this operation.
	Parameters []parameter

	Constants map[string]string

	// Possible responses returned by the operation,
	// keyed by HTTP status code.
	Responses map[int]reflect.Type

	argumentsNeedsMapping bool
}

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

// specification describes a rest API specification.
type specification struct {
	api.Structure

	Resources map[string]resource `api:"-"`

	duplicates []error
}

func specificationOf(rest api.Structure) (specification, error) {
	var spec specification
	if err := spec.setSpecification(rest); err != nil {
		return specification{}, err
	}
	return spec, nil
}

func (spec *specification) setSpecification(to api.Structure) error {
	spec.Structure = to
	return spec.load(to)
}

func (spec *specification) load(from api.Structure) error {
	for _, fn := range from.Functions {
		if err := spec.loadOperation(fn); err != nil {
			return xray.New(err)
		}
	}
	for _, section := range from.Namespace {
		if err := spec.load(section); err != nil {
			return xray.New(err)
		}
	}
	return nil
}

func (spec *specification) makeResponses(fn api.Function) (map[int]reflect.Type, error) {
	var responses = make(map[int]reflect.Type)
	var (
		rules = rtags.ResultRulesOf(string(fn.Tags.Get("rest")))
	)
	if len(rules) == 0 && fn.NumOut() == 1 {
		responses[200] = fn.Type.Out(0)
		return responses, nil
	}
	var (
		p = newParser(fn)
	)
	if len(rules) != fn.NumOut() {
		return nil, fmt.Errorf("%s result rules must match the number of return values", p.debug())
	}
	var fields []reflect.StructField
	for i := 0; i < fn.NumOut(); i++ {
		fields = append(fields, reflect.StructField{
			Name: strings.Title(rules[i]),
			Tag:  reflect.StructTag(`json:"` + rules[i] + `"`),
			Type: fn.Type.Out(i),
		})
	}
	responses[200] = reflect.StructOf(fields)
	return responses, nil
}

func (spec *specification) loadOperation(fn api.Function) error {
	tag := string(fn.Tags.Get("rest"))
	if tag == "-" || tag == "" {
		return nil //skip
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
	var ContentType = "application/json" // default content type is JSON, unless otherwise specified.
	if strings.Contains(method, "(") {
		if !strings.HasSuffix(method, ")") {
			return fmt.Errorf("make sure the %s 'rest' ContentType has a closing bracket", fn.Name)
		}
		method = strings.TrimSuffix(method, ")")
		method, ContentType, _ = strings.Cut(method, "(")
	}
	var (
		params = newParser(fn)
		args   []reflect.Type
	)
	for i := 0; i < fn.NumIn(); i++ {
		arg := fn.In(i)
		args = append(args, arg)
	}
	splits = strings.SplitN(splits[1], "?", 2)
	path = splits[0]
	if err := params.parsePath(path, args); err != nil {
		return xray.New(err)
	}
	path = strings.ReplaceAll(path, "=%v", "")
	if len(splits) > 1 {
		query = "?" + splits[1]
		if err := params.parseQuery(query, args); err != nil {
			return xray.New(err)
		}
	}
	if err := params.parseBody(rtags.ArgumentRulesOf(tag)); err != nil {
		return xray.New(err)
	}
	responses, err := spec.makeResponses(fn)
	if err != nil {
		return xray.New(err)
	}
	res := spec.Resources[path]
	if res.Operations == nil {
		res.Operations = make(map[http_api.Method]operation)
	}
	res.Name = fn.Name
	// If two names collide, this is probably a mistake and we want to return an error.
	if existing, ok := res.Operations[http_api.Method(method)]; ok {
		spec.duplicates = append(spec.duplicates, fmt.Errorf("by deduplicating the duplicate endpoint '%s %s' (%s and %s)",
			method, path, strings.Join(append(existing.Path, existing.Name), "."), strings.Join(append(fn.Path, fn.Name), ".")))
	}
	var argumentsNeedsMapping = false
	var count int
	for _, param := range params.list {
		if param.Location == parameterInBody {
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
	res.Operations[http_api.Method(method)] = operation{
		Function:   fn,
		Parameters: params.list,
		Constants:  params.static,
		Responses:  responses,

		DefaultContentType: oas.ContentType(ContentType),

		argumentsNeedsMapping: argumentsNeedsMapping,
	}
	if spec.Resources == nil {
		spec.Resources = make(map[string]resource)
	}
	spec.Resources[path] = res
	return nil
}

type parser struct {
	pos int

	list []parameter

	static map[string]string

	fn api.Function
}

func newParser(fn api.Function) *parser {
	return &parser{
		list:   make([]parameter, fn.NumIn()),
		static: make(map[string]string),
		fn:     fn,
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
			if ok {
				p.static[name] = format
			}
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
			return xray.New(err)
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
		// is represented with a '%v', ie. GET /path/to/endpoint?%v
		// The name of this should map to the type in the function arguments.
		destructure := func(i int) error {
			if i >= len(args) {
				return fmt.Errorf("%s destructured parameter %d is out of range", p.debug(), i)
			}
			var (
				arg = args[i]
			)
			for arg.Kind() == reflect.Ptr {
				arg = arg.Elem()
			}
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
						name, _, _ = strings.Cut(field.Tag.Get("json"), ",")
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
					if field.Anonymous {
						walk(field.Type, append(index, field.Index...), parent)
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
			p.list[p.pos].Location = parameterInVoid
			// walk through and destructure each field as a
			// query parameter.
			if err := destructure(p.pos); err != nil {
				return xray.New(err)
			}
			p.pos++
		} else {
			if err := p.parseParam(param, args, parameterInQuery); err != nil {
				return xray.New(err)
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
			if name := field.Tag.Get("json"); name == param {
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
				return xray.New(err)
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
