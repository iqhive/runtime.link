package api

import (
	"fmt"
	"path"
	"reflect"
	"strings"

	"runtime.link/has"
	"runtime.link/ref/oss/license"
	"runtime.link/ref/std/email"
	"runtime.link/ref/std/media"
	"runtime.link/ref/std/uri"
	"runtime.link/ref/std/url"
	"runtime.link/txt/std/human"
	"runtime.link/txt/std/markdown"
)

type openapi string

// Documentation should be embedded in all runtime.link API structures. When an API is imported
// or exported, the documentation will be filled in. Is an extension of the OpenAPI specification.
type Documentation struct {
	OpenAPI    openapi                          `json:"openapi"`
	Details    Details                          `json:"info"`
	Dialect    uri.String                       `json:"jsonSchemaDialect,omitempty"`
	Servers    []oasServer                      `json:"servers,omitempty"`
	Functions  map[string]oasPathItem           `json:"paths,omitempty"`
	Callbacks  map[string]*oasPathItem          `json:"webhooks,omitempty"`
	Resources  *oasComponents                   `json:"components,omitempty"`
	Security   []map[string]oasSecuritySchemeID `json:"security,omitempty"`
	Namespaces []oasTag                         `json:"tags,omitempty"`
	SeeAlso    *oasExternalDocumentation        `json:"externalDocs,omitempty"`
}

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

// DocumentationOf returns a [Specification] for a [Structure].
func DocumentationOf(structure Structure) (Documentation, error) {
	var spec Documentation
	spec.OpenAPI = "3.1.0"
	for _, fn := range structure.Functions {
		rest := fn.Tags.Get("rest")
		method, path, ok := strings.Cut(rest, " ")
		if !ok {
			return spec, fmt.Errorf("invalid rest tag: %q", rest)
		}
		var operation oasFunction
		operation.ID = oasFunctionID(fn.Name)
		operation.Summary = human.Readable(fn.Name)
		var item oasPathItem
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
		if spec.Functions == nil {
			spec.Functions = make(map[string]oasPathItem)
		}
		if fn.NumIn() == 1 {
			var body oasRequestBody
			var mtype oasMediaType
			schema := spec.SchemaFor(fn.In(0))
			mtype.Schema = schema
			body.Content = make(map[media.Type]oasMediaType)
			body.Content["application/json"] = mtype
			operation.RequestBody = &body
		}
		spec.Functions[path] = item
	}
	return spec, nil
}

// SchemaFor returns a [Schema] for a Go value.
func (spec *Documentation) SchemaFor(val any) *oasSchema {
	rtype, ok := val.(reflect.Type)
	if !ok {
		rtype = reflect.TypeOf(val)
	}
	var useRef bool
	var schema oasSchema
	if rtype.PkgPath() != "" {
		schema.Title = human.Readable(rtype.Name())
		useRef = true
	}
	switch rtype.Kind() {
	case reflect.Bool:
		schema.Type = []oasType{oasTypes.Bool}
	case reflect.Int8:
		schema.Type = []oasType{oasTypes.Integer}
		schema.Min = has.New(-128.0)
		schema.Max = has.New(127.0)
	case reflect.Int16:
		schema.Type = []oasType{oasTypes.Integer}
		schema.Min = has.New(-32768.0)
		schema.Max = has.New(32767.0)
	case reflect.Int32:
		schema.Type = []oasType{oasTypes.Integer}
		schema.Min = has.New(-2147483648.0)
		schema.Max = has.New(2147483647.0)
	case reflect.Int64:
		schema.Type = []oasType{oasTypes.Integer}
		schema.Min = has.New(-9223372036854775808.0)
		schema.Max = has.New(9223372036854775807.0)
	case reflect.Int:
		schema.Type = []oasType{oasTypes.Integer}
	case reflect.Uint8:
		schema.Type = []oasType{oasTypes.Integer}
		schema.Min = has.New(0.0)
		schema.Max = has.New(255.0)
	case reflect.Uint16:
		schema.Type = []oasType{oasTypes.Integer}
		schema.Min = has.New(0.0)
		schema.Max = has.New(65535.0)
	case reflect.Uint32:
		schema.Type = []oasType{oasTypes.Integer}
		schema.Min = has.New(0.0)
		schema.Max = has.New(4294967295.0)
	case reflect.Uint64:
		schema.Type = []oasType{oasTypes.Integer}
		schema.Min = has.New(0.0)
		schema.Max = has.New(18446744073709551615.0)
	case reflect.Uint, reflect.Uintptr:
		schema.Type = []oasType{oasTypes.Integer}
		schema.Min = has.New(0.0)
	case reflect.Float32, reflect.Float64:
		schema.Type = []oasType{oasTypes.Number}
	case reflect.String:
		schema.Type = []oasType{oasTypes.String}
	case reflect.Map:
		schema.Type = []oasType{oasTypes.Object}
		schema.PropertyNames = spec.SchemaFor(rtype.Key())
		schema.AdditionalProperties = spec.SchemaFor(rtype.Elem())
	case reflect.Pointer:
		return spec.SchemaFor(rtype.Elem())
	case reflect.Slice:
		schema.Type = []oasType{oasTypes.Array}
		schema.Items = spec.SchemaFor(rtype.Elem())
	case reflect.Struct:
		schema.Type = []oasType{oasTypes.Object}
		schema.Properties = make(map[oasPropertyName]*oasSchema)
		for i := 0; i < rtype.NumField(); i++ {
			field := rtype.Field(i)
			if field.PkgPath != "" {
				continue
			}
			var name oasPropertyName
			if field.Tag != "" {
				tag, _, _ := strings.Cut(field.Tag.Get("json"), ",")
				name = oasPropertyName(tag)
			}
			if name == "" {
				name = oasPropertyName(field.Name)
			}
			description := documentationOf(field.Tag)
			if field.Type == reflect.TypeOf(has.Documentation{}) {
				schema.Description = human.Readable(documentationOf(rtype.Field(0).Tag))
				continue
			}
			var property = spec.SchemaFor(field.Type)
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
			schema.Min = has.New(float64(val.Int()))
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
			schema.Min = has.New(float64(val.Uint()))
		case reflect.Float32, reflect.Float64:
			schema.Min = has.New(val.Float())
		}
	}
	max, ok := rtype.MethodByName("Max")
	if ok {
		val := max.Func.Call([]reflect.Value{reflect.Zero(rtype)})[0]
		switch rtype.Kind() {
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			schema.Max = has.New(float64(val.Int()))
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
			schema.Max = has.New(float64(val.Uint()))
		case reflect.Float32, reflect.Float64:
			schema.Max = has.New(val.Float())
		}
	}
	if useRef {
		if spec.Resources == nil {
			spec.Resources = &oasComponents{}
		}
		if spec.Resources.Schemas == nil {
			spec.Resources.Schemas = make(map[string]*oasSchema)
		}
		pkg, ok := spec.Resources.Schemas[path.Base(rtype.PkgPath())]
		if !ok {
			pkg = &oasSchema{}
		}
		if pkg.Defs == nil {
			pkg.Defs = make(map[string]*oasSchema)
		}
		pkg.Defs[rtype.Name()] = &schema
		spec.Resources.Schemas[path.Base(rtype.PkgPath())] = pkg
		return &schema
	}
	return &schema
}
