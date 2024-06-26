package api

import (
	"fmt"
	"path"
	"reflect"
	"strings"

	"runtime.link/api/internal/has"
	"runtime.link/api/internal/oas"
	"runtime.link/api/internal/rtags"
	"runtime.link/api/xray"
)

// oasDocumentOf returns a [oas.Document] for a [Structure].
func oasDocumentOf(structure Structure) (oas.Document, error) {
	var spec oas.Document
	spec.OpenAPI = "3.1.0"
	for _, fn := range structure.Functions {
		if err := addFunctionTo(&spec, fn); err != nil {
			return spec, xray.Error(err)
		}
	}
	return spec, nil
}

func addFunctionTo(spec *oas.Document, fn Function) error {
	path := fn.Tags.Get("rest")
	if path == "" {
		path = fn.Tags.Get("http")
	}
	method, path, ok := strings.Cut(path, " ")
	if !ok {
		return fmt.Errorf("invalid tag: %q", path)
	}
	operation, err := operationFor(spec, fn, path)
	if err != nil {
		return xray.Error(err)
	}
	var item oas.PathItem
	switch method {
	case "GET":
		item.Get = &operation
	case "PUT":
		item.Put = &operation
	case "POST":
		item.Post = &operation
	case "DELETE":
		item.Delete = &operation
	case "OPTIONS":
		item.Options = &operation
	case "HEAD":
		item.Head = &operation
	case "PATCH":
		item.Patch = &operation
	case "TRACE":
		item.Trace = &operation
	default:
		return fmt.Errorf("invalid rest method: %q", method)
	}
	if spec.Paths == nil {
		spec.Paths = make(map[string]oas.PathItem)
	}
	spec.Paths[path] = item
	return nil
}

// operationOf returns a [oas.Operation] for a [Function].
func operationFor(spec *oas.Document, fn Function, path string) (oas.Operation, error) {
	var operation oas.Operation
	operation.ID = oas.OperationID(fn.Name)
	operation.Summary = oas.Readable(fn.Name)
	var (
		params = newParser(fn)
		args   []reflect.Type
	)
	for i := 0; i < fn.NumIn(); i++ {
		arg := fn.In(i)
		args = append(args, arg)
	}
	path, query, ok := strings.Cut(path, "?")
	if err := params.parsePath(path, args); err != nil {
		return operation, xray.Error(err)
	}
	path = strings.ReplaceAll(path, "=%v", "")
	if ok {
		query = "?" + query
		if err := params.parseQuery(query, args); err != nil {
			return operation, xray.Error(err)
		}
	}
	argumentRules := rtags.ArgumentRulesOf(fn.Tags.Get("txt"))
	if err := params.parseBody(argumentRules); err != nil {
		return operation, xray.Error(err)
	}
	/*responses, err := spec.makeResponses(fn)
	if err != nil {
		return xray.Error(err)
	}*/
	var bodyArg int
	var bodyMapping = make(map[string]oas.Schema)
	var bodyArguments int
	for _, param := range params.list {
		if param.Location == parameterInBody {
			bodyArguments++
		}
	}
	for i, arg := range params.list {
		var param oas.Parameter
		param.Schema = schemaFor(spec, arg.Type)
		switch arg.Location {
		case parameterInPath:
			param.In = oas.ParameterLocations.Path
		case parameterInQuery:
			param.In = oas.ParameterLocations.Query
		case parameterInBody:
			if bodyArguments > 1 {
				bodyMapping[argumentRules[i]] = *param.Schema
			} else {
				bodyArg = i
			}
			continue
		}
		operation.Parameters = append(operation.Parameters, &param)
	}
	if len(bodyMapping) == 0 {
		var body oas.RequestBody
		body.Content = make(map[oas.ContentType]oas.MediaType)
		var applicationJSON = oas.ContentType("application/json")
		body.Content[applicationJSON] = oas.MediaType{
			Schema: schemaFor(spec, fn.In(bodyArg)),
		}
		operation.RequestBody = &body
	}
	return operation, nil
}

// schemaFor returns a [Schema] for a Go value.
func schemaFor(spec *oas.Document, val any) *oas.Schema {
	rtype, ok := val.(reflect.Type)
	if !ok {
		rtype = reflect.TypeOf(val)
	}
	var useRef bool
	var schema oas.Schema
	if rtype.PkgPath() != "" {
		schema.Title = oas.Readable(rtype.Name())
		useRef = true
	}
	switch rtype.Kind() {
	case reflect.Bool:
		schema.Type = []oas.Type{oas.Types.Bool}
	case reflect.Int8:
		schema.Type = []oas.Type{oas.Types.Integer}
		schema.Minimum = has.New(-128.0)
		schema.Maximum = has.New(127.0)
	case reflect.Int16:
		schema.Type = []oas.Type{oas.Types.Integer}
		schema.Minimum = has.New(-32768.0)
		schema.Maximum = has.New(32767.0)
	case reflect.Int32:
		schema.Type = []oas.Type{oas.Types.Integer}
		schema.Minimum = has.New(-2147483648.0)
		schema.Maximum = has.New(2147483647.0)
	case reflect.Int64:
		schema.Type = []oas.Type{oas.Types.Integer}
		schema.Minimum = has.New(-9223372036854775808.0)
		schema.Maximum = has.New(9223372036854775807.0)
	case reflect.Int:
		schema.Type = []oas.Type{oas.Types.Integer}
	case reflect.Uint8:
		schema.Type = []oas.Type{oas.Types.Integer}
		schema.Minimum = has.New(0.0)
		schema.Maximum = has.New(255.0)
	case reflect.Uint16:
		schema.Type = []oas.Type{oas.Types.Integer}
		schema.Minimum = has.New(0.0)
		schema.Maximum = has.New(65535.0)
	case reflect.Uint32:
		schema.Type = []oas.Type{oas.Types.Integer}
		schema.Minimum = has.New(0.0)
		schema.Maximum = has.New(4294967295.0)
	case reflect.Uint64:
		schema.Type = []oas.Type{oas.Types.Integer}
		schema.Minimum = has.New(0.0)
		schema.Maximum = has.New(18446744073709551615.0)
	case reflect.Uint, reflect.Uintptr:
		schema.Type = []oas.Type{oas.Types.Integer}
		schema.Minimum = has.New(0.0)
	case reflect.Float32, reflect.Float64:
		schema.Type = []oas.Type{oas.Types.Number}
	case reflect.String:
		schema.Type = []oas.Type{oas.Types.String}
	case reflect.Map:
		schema.Type = []oas.Type{oas.Types.Object}
		schema.PropertyNames = schemaFor(spec, rtype.Key())
		schema.AdditionalProperties = schemaFor(spec, rtype.Elem())
	case reflect.Pointer:
		return schemaFor(spec, rtype.Elem())
	case reflect.Slice:
		schema.Type = []oas.Type{oas.Types.Array}
		schema.Items = schemaFor(spec, rtype.Elem())
	case reflect.Struct:
		schema.Type = []oas.Type{oas.Types.Object}
		schema.Properties = make(map[oas.PropertyName]*oas.Schema)
		for i := 0; i < rtype.NumField(); i++ {
			field := rtype.Field(i)
			if field.PkgPath != "" {
				continue
			}
			var name oas.PropertyName
			if field.Tag != "" {
				tag, _, _ := strings.Cut(field.Tag.Get("json"), ",")
				name = oas.PropertyName(tag)
			}
			if name == "" {
				name = oas.PropertyName(field.Name)
			}
			description := documentationOf(field.Tag)
			if field.Type == reflect.TypeOf(has.Documentation{}) {
				schema.Description = oas.Readable(documentationOf(rtype.Field(0).Tag))
				continue
			}
			var property = schemaFor(spec, field.Type)
			if description != "" {
				if description[0] == '(' {
					property.Description = oas.Readable(description[1 : len(description)-1])
				} else {
					property.Description = oas.Readable(fmt.Sprintf("%s %s", field.Name, description))
				}
			}
			schema.Properties[name] = property
			if !strings.Contains(string(field.Tag), ",omitempty") {
				schema.Required = append(schema.Required, name)
			}
		}
	}
	min, ok := rtype.MethodByName("Min")
	if ok {
		val := min.Func.Call([]reflect.Value{reflect.Zero(rtype)})[0]
		switch rtype.Kind() {
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			schema.Minimum = has.New(float64(val.Int()))
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
			schema.Minimum = has.New(float64(val.Uint()))
		case reflect.Float32, reflect.Float64:
			schema.Minimum = has.New(val.Float())
		}
	}
	max, ok := rtype.MethodByName("Max")
	if ok {
		val := max.Func.Call([]reflect.Value{reflect.Zero(rtype)})[0]
		switch rtype.Kind() {
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			schema.Maximum = has.New(float64(val.Int()))
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
			schema.Maximum = has.New(float64(val.Uint()))
		case reflect.Float32, reflect.Float64:
			schema.Maximum = has.New(val.Float())
		}
	}
	if useRef {
		if spec.Components == nil {
			spec.Components = &oas.Components{}
		}
		if spec.Components.Schemas == nil {
			spec.Components.Schemas = make(map[string]*oas.Schema)
		}
		pkg, ok := spec.Components.Schemas[path.Base(rtype.PkgPath())]
		if !ok {
			pkg = &oas.Schema{}
		}
		if pkg.Defs == nil {
			pkg.Defs = make(map[string]*oas.Schema)
		}
		pkg.Defs[rtype.Name()] = &schema
		spec.Components.Schemas[path.Base(rtype.PkgPath())] = pkg
		return &schema
	}
	return &schema
}
