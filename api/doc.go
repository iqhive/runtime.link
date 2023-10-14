package api

import (
	"fmt"
	"path"
	"reflect"
	"strings"

	"runtime.link/has"
	"runtime.link/oas"
	"runtime.link/ref/oss/license"
	"runtime.link/ref/std/email"
	"runtime.link/ref/std/media"
	"runtime.link/ref/std/url"
	"runtime.link/txt/std/human"
	"runtime.link/txt/std/markdown"
)

type openapi string

// Specification should be embedded in all runtime.link API structures.
type Specification struct{}

// Version string for an API.
type Version string

// Details about an API.
type Details struct {
	Name            human.Readable  `json:"title"`
	Summary         human.Readable  `json:"summary,omitempty"`
	Version         Version         `json:"version"`
	License         *Licensing      `json:"license,omitempty"`
	TermsConditions url.String      `json:"termsOfService,omitempty"`
	Description     markdown.String `json:"description,omitempty"`
	Contact         *ContactDetails `json:"contact,omitempty"`
}

// Licensing information for the implementation of the API.
type Licensing struct {
	ID   license.ID     `json:"identifier,omitempty"`
	Name human.Readable `json:"name"`
	URL  url.String     `json:"url,omitempty"`
}

// ContactDetails for an API.
type ContactDetails struct {
	Name  human.Name    `json:"name,omitempty"`
	URL   url.String    `json:"url,omitempty"`
	Email email.Address `json:"email,omitempty"`
}

// DocumentationOf returns a [oas.Document] for a [Structure].
func DocumentationOf(structure Structure) (oas.Document, error) {
	var spec oas.Document
	spec.OpenAPI = "3.1.0"
	for _, fn := range structure.Functions {
		rest := fn.Tags.Get("rest")
		method, path, ok := strings.Cut(rest, " ")
		if !ok {
			return spec, fmt.Errorf("invalid rest tag: %q", rest)
		}
		var operation oas.Function
		operation.ID = oas.FunctionID(fn.Name)
		operation.Summary = human.Readable(fn.Name)
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
			return spec, fmt.Errorf("invalid rest method: %q", method)
		}
		if spec.Paths == nil {
			spec.Paths = make(map[string]oas.PathItem)
		}
		if fn.NumIn() == 1 {
			var body oas.RequestBody
			var mtype oas.MediaType
			schema := schemaFor(&spec, fn.In(0))
			mtype.Schema = schema
			body.Content = make(map[media.Type]oas.MediaType)
			body.Content["application/json"] = mtype
			operation.RequestBody = &body
		}
		spec.Paths[path] = item
	}
	return spec, nil
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
		schema.Title = human.Readable(rtype.Name())
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
				schema.Description = human.Readable(documentationOf(rtype.Field(0).Tag))
				continue
			}
			var property = schemaFor(spec, field.Type)
			if description != "" {
				if description[0] == '(' {
					property.Description = human.Readable(description[1 : len(description)-1])
				} else {
					property.Description = human.Readable(fmt.Sprintf("%s %s", field.Name, description))
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
