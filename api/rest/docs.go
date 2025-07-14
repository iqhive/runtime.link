package rest

import (
	"bytes"
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"

	"runtime.link/api"
	"runtime.link/api/internal/has"
	"runtime.link/api/internal/oas"
	"runtime.link/api/internal/rtags"
	"runtime.link/api/xray"
	"runtime.link/pii/email"
	"runtime.link/xyz"
)

func formatPascalCaseTitle(name string) string {
	var result strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune(' ')
		}
		result.WriteRune(r)
	}
	return result.String()
}

func handleDocs(r *http.Request, w http.ResponseWriter, wrap func(error) error, impl any) {
	w.Write([]byte("<!DOCTYPE html>"))
	w.Write(docs_head)
	w.Write([]byte("<body>"))
	if documented, ok := impl.(api.WithExamples); ok {
		examples, err := documented.Examples(r.Context())
		if err == nil {
			w.Write([]byte("<nav>"))
			fmt.Fprintf(w, "<h2><a href=''>API Reference</a></h2>")
			w.Write([]byte("<h3>Examples</h3>"))
			
			w.Write([]byte("<div class=\"examples-list\">"))
			for category, categoryExamples := range examples {
				fmt.Fprintf(w, "<details class=\"example-category\">")
				fmt.Fprintf(w, "<summary class=\"category-header\">%s</summary>", strings.Title(category))
				fmt.Fprintf(w, "<div class=\"category-examples\">")
				for _, exampleName := range categoryExamples {
					title := formatPascalCaseTitle(exampleName)
					fmt.Fprintf(w, "<a href=\"./examples/%v\" class=\"example-link\">%s</a>", exampleName, title)
				}
				fmt.Fprintf(w, "</div></details>")
			}
			w.Write([]byte("</div></nav>"))
		}
	}
	w.Write([]byte("<main id='swagger-ui'>"))
	w.Write(docs_body)
	w.Write([]byte("</main></body></html>"))
}


func sample(fn api.Function, args, rets []reflect.Value) (url string, req, resp []byte, err error) {
	var spec specification
	if err := spec.loadOperation(fn); err != nil {
		return "", nil, nil, xray.New(err)
	}
	// there will only be one.
	for path, resource := range spec.Resources {
		for method, operation := range resource.Operations {
			var body bytes.Buffer
			var path, _, _ = operation.clientWrite(make(http.Header), path, args, &body, true)
			var resp bytes.Buffer

			if method == "GET" {
				body.Reset()
			}

			enc := json.NewEncoder(&resp)
			enc.SetIndent("", "\t")

			var rules = rtags.ResultRulesOf(string(fn.Tags.Get("rest")))
			if rules != nil {
				var mapping = make(map[string]json.RawMessage)
				for i, result := range rets {
					msg, _ := json.MarshalIndent(result.Interface(), "", "\t")
					mapping[rules[i]] = json.RawMessage(msg)
				}
				enc.Encode(mapping)
			} else if len(rets) == 1 {
				enc.Encode(rets[0].Interface())
			}

			return string(method) + " " + path, body.Bytes(), resp.Bytes(), nil
		}
	}
	return
}

// oasDocumentOf returns a [oas.Document] for a [Structure].
func oasDocumentOf(structure api.Structure) (oas.Document, error) {
	var spec oas.Document
	spec.OpenAPI = "3.1.0"
	if structure.Name != "" {
		spec.Information.Title = oas.Readable(structure.Name) + " API"
	}
	if structure.Docs != "" {
		spec.Information.Description = oas.Markdown("This API " + structure.Docs)
	}
	for _, fn := range structure.Functions {
		if err := addFunctionTo(&spec, fn, "default"); err != nil {
			return spec, xray.New(err)
		}
	}
	for name, ns := range structure.Namespace {
		if ns.Tags.Get("swagger") == "-" || ns.Tags.Get("docs") == "-" || ns.Tags.Get("openapi") == "-" {
			continue
		}
		if err := addNamespaceTo(&spec, name, ns); err != nil {
			return spec, xray.New(err)
		}
	}
	return spec, nil
}

// addNamespaceTo adds a namespace to a [oas.Document].
func addNamespaceTo(spec *oas.Document, name string, ns api.Structure) error {
	for _, fn := range ns.Functions {
		if err := addFunctionTo(spec, fn, name); err != nil {
			return xray.New(err)
		}
	}
	for _, ns := range ns.Namespace {
		if ns.Tags.Get("swagger") == "-" || ns.Tags.Get("docs") == "-" || ns.Tags.Get("openapi") == "-" {
			continue
		}
		if err := addNamespaceTo(spec, name, ns); err != nil {
			return xray.New(err)
		}
	}
	return nil
}

func addFunctionTo(spec *oas.Document, fn api.Function, namespace string) error {
	path := fn.Tags.Get("rest")
	if path == "" {
		path = fn.Tags.Get("http")
	}
	if path == "" || path == "-" {
		return nil
	}
	if fn.Tags.Get("swagger") == "-" || fn.Tags.Get("docs") == "-" || fn.Tags.Get("openapi") == "-" {
		return nil
	}
	method, path, ok := strings.Cut(path, " ")
	if !ok {
		return fmt.Errorf("invalid tag: %q", path)
	}
	operation, err := operationFor(spec, fn, path)
	if err != nil {
		return xray.New(err)
	}
	operation.Tags = append(operation.Tags, namespace)
	path, _, _ = strings.Cut(path, " ")
	path, _, _ = strings.Cut(path, "?")
	method, mime, ok := strings.Cut(method, "(")
	if ok {
		mime = strings.TrimSuffix(mime, ")")
	}
	path = rtags.CleanupPattern(path)
	var item = spec.Paths[path]
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
func operationFor(spec *oas.Document, fn api.Function, path string) (oas.Operation, error) {
	var operation oas.Operation
	operation.ID = oas.OperationID(fn.Name)
	operation.Summary = oas.Readable(fn.Name)
	operation.Description = oas.Readable(fn.Name + " " + fn.Docs)
	var (
		params = newParser(fn)
		args   []reflect.Type
	)
	for i := 0; i < fn.NumIn(); i++ {
		arg := fn.In(i)
		args = append(args, arg)
	}
	path, _, _ = strings.Cut(path, " ")
	path, query, ok := strings.Cut(path, "?")
	if err := params.parsePath(path, args); err != nil {
		return operation, xray.New(err)
	}
	path = strings.ReplaceAll(path, "=%v", "")
	if ok {
		query = "?" + query
		if err := params.parseQuery(query, args); err != nil {
			return operation, xray.New(err)
		}
	}
	argumentRules := rtags.ArgumentRulesOf(fn.Tags.Get("rest"))
	resultRules := rtags.ResultRulesOf(fn.Tags.Get("rest"))
	var argumentRule int
	if err := params.parseBody(argumentRules); err != nil {
		return operation, xray.New(err)
	}
	/*responses, err := spec.makeResponses(fn)
	if err != nil {
		return xray.Error(err)
	}*/
	var bodyArg int = -1
	var bodyMapping = make(map[oas.PropertyName]*oas.Schema)
	var bodyArguments int
	for _, param := range params.list {
		if param.Location == parameterInBody {
			bodyArguments++
		}
	}
	for i, arg := range params.list {
		if arg.Location < 0 {
			continue
		}
		var param oas.Parameter
		param.Name = oas.Readable(arg.Name)
		param.Schema = schemaFor(spec, arg.Type)
		switch arg.Location {
		case parameterInPath:
			param.In = oas.ParameterLocations.Path
			param.Style = oas.ParameterStyles.Simple
			param.Required = true
		case parameterInQuery:
			param.In = oas.ParameterLocations.Query
			param.Style = oas.ParameterStyles.Form
		case parameterInBody:
			if len(argumentRules) > 0 {
				if len(argumentRules) <= argumentRule {
					return operation, fmt.Errorf("not enough argument rules for %q", fn.Name)
				}
				bodyMapping[oas.PropertyName(argumentRules[argumentRule])] = param.Schema
				argumentRule++
			} else {
				bodyArg = i
			}
			continue
		}
		operation.Parameters = append(operation.Parameters, &param)
	}
	var bodySchema *oas.Schema
	if len(bodyMapping) == 0 && bodyArg != -1 {
		btype := fn.In(bodyArg)
		switch btype {
		case reflect.TypeFor[io.Reader](), reflect.TypeFor[io.ReadCloser](), reflect.TypeFor[io.WriterTo](), reflect.TypeFor[io.LimitedReader]():
			mime := fn.Tags.Get("mime")
			if mime == "" {
				mime = "application/octet-stream"
			}
			content_types := map[oas.ContentType]oas.MediaType{}
			for _, mime := range strings.Split(mime, ",") {
				content_types[oas.ContentType(mime)] = oas.MediaType{}
			}
			operation.RequestBody = &oas.RequestBody{
				Content: content_types,
			}
		default:
			bodySchema = schemaFor(spec, btype)
		}

	} else if len(bodyMapping) > 0 {
		bodySchema = &oas.Schema{
			Type:       oas.TypeSet{oas.Types.Object},
			Properties: bodyMapping,
		}
	}
	if bodySchema != nil {
		var body oas.RequestBody
		body.Content = make(map[oas.ContentType]oas.MediaType)
		var applicationJSON = oas.ContentType("application/json")
		body.Content[applicationJSON] = oas.MediaType{
			Schema: bodySchema,
		}
		operation.RequestBody = &body
	}
	var respSchema *oas.Schema
	if fn.NumOut() == 1 && len(resultRules) == 0 {
		respSchema = schemaFor(spec, fn.Type.Out(0))
	} else if fn.NumOut() > 0 || len(resultRules) > 0 {
		results := make(map[oas.PropertyName]*oas.Schema)
		for i := 0; i < fn.NumOut(); i++ {
			results[oas.PropertyName(resultRules[i])] = schemaFor(spec, fn.Type.Out(i))
		}
		respSchema = &oas.Schema{
			Type:       oas.TypeSet{oas.Types.Object},
			Properties: results,
		}
	}
	if respSchema != nil {
		operation.Responses = make(map[oas.ResponseKey]*oas.Response)
		var response oas.Response
		response.Content = make(map[oas.ContentType]oas.MediaType)
		var applicationJSON = oas.ContentType("application/json")
		response.Content[applicationJSON] = oas.MediaType{
			Schema: respSchema,
		}
		operation.Responses[oas.ResponseKeys.Default] = &response
	}
	return operation, nil
}

func addFieldsToSchema(schema *oas.Schema, reg oas.Registry, rtype reflect.Type) {
	var processed = make(map[oas.PropertyName]bool)
	var anonymous []reflect.Type
	for i := 0; i < rtype.NumField(); i++ {
		field := rtype.Field(i)
		if field.PkgPath != "" {
			continue
		}
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			anonymous = append(anonymous, field.Type)
			continue
		}
		var name oas.PropertyName
		if field.Tag != "" {
			tag, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			if tag == "-" {
				continue
			}
			name = oas.PropertyName(tag)
		}
		if name == "" {
			name = oas.PropertyName(field.Name)
		}
		description := api.DocumentationOf(field)
		if field.Type == reflect.TypeOf(has.Documentation{}) {
			schema.Description = oas.Readable(api.DocumentationOf(rtype.Field(0)))
			continue
		}
		var property = schemaFor(reg, field.Type)
		property.Title = oas.Readable(field.Name)
		if description != "" {
			if description[0] == '(' {
				property.Description = oas.Readable(description[1 : len(description)-1])
			}
			property.Description = oas.Readable(description)
		}
		schema.Properties[name] = property
		if !strings.Contains(string(field.Tag), ",omitempty") && !strings.Contains(string(field.Tag), ",omitzero") && field.Type.Kind() != reflect.Bool && field.Type.Kind() != reflect.Pointer {
			schema.Required = append(schema.Required, name)
		}
		if constString, ok := field.Tag.Lookup("const"); ok && field.Type.Kind() == reflect.String {
			property.Const = json.RawMessage(strconv.Quote(constString))
		}
		processed[name] = true
	}
	for _, embedded := range anonymous {
		if _, ok := processed[oas.PropertyName(embedded.Name())]; !ok {
			addFieldsToSchema(schema, reg, embedded)
		}
	}
}

func formatFor(rtype reflect.Type) *oas.Format {
	switch reflect.Zero(rtype).Interface().(type) {
	case time.Time:
		return &oas.Formats.DateTime
	case email.Address:
		return &oas.Formats.Email
	case int32:
		return &oas.Formats.Int32
	case int64:
		return &oas.Formats.Int64
	case float32:
		return &oas.Formats.Float
	case float64:
		return &oas.Formats.Double
	default:
		namespace, name := namespaceName(rtype)
		if namespace == "." {
			format := xyz.Raw[oas.Format](name)
			return &format
		}
		format := xyz.Raw[oas.Format](namespace + "." + name)
		return &format
	}
}

func cleanup(name string) string {
	parent, child, ok := strings.Cut(name, "[")
	if ok {
		child = strings.TrimSuffix(child, "]")
		renew := ""
		for _, split := range strings.Split(child, ",") {
			if renew != "" {
				renew += ", "
			}
			renew += cleanup(split)
		}
		child = renew
	}
	if strings.Contains(parent, "/") {
		parent = path.Base(parent)
	}
	parent, _, _ = strings.Cut(parent, "·")
	if ok {
		return parent + "[" + child + "]"
	}
	return parent
}

func namespaceName(rtype reflect.Type) (string, string) {
	namespace, name := path.Base(rtype.PkgPath()), rtype.Name()
	return namespace, cleanup(name)
}

// schemaFor returns a [Schema] for a Go value.
func schemaFor(reg oas.Registry, val any) *oas.Schema {
	if val == nil {
		return nil
	}
	rtype, isType := val.(reflect.Type)
	if !isType {
		rtype = reflect.TypeOf(val)
	}
	nitfc := reflect.New(rtype).Interface()
	if jtype, ok := nitfc.(interface {
		TypeJSON() reflect.Type
	}); ok {
		return schemaFor(reg, jtype.TypeJSON())
	}
	if utype, ok := nitfc.(interface {
		TypesJSON() []reflect.Type
	}); ok {
		var oneof []*oas.Schema
		for _, t := range utype.TypesJSON() {
			oneof = append(oneof, schemaFor(reg, t))
		}
		return &oas.Schema{
			OneOf: oneof,
		}
	}
	namespace, name := namespaceName(rtype)
	if reg != nil {
		if existing := reg.Lookup(namespace, name); existing != nil {
			existing.Format = formatFor(rtype)
			return existing
		}
	}
	var useRef bool
	schema := new(oas.Schema)
	if rtype.PkgPath() != "" {
		schema.Title = oas.Readable(rtype.Name())
		useRef = true
	}
	if reg == nil {
		reg = schema
		useRef = false
	}
	if useRef && schema != reg {
		reg.Register(namespace, name, schema)
	}
	schema.Format = formatFor(rtype)
	if exemplar, ok := nitfc.(interface {
		Example()
	}); ok {
		exemplar.Example()
		example, err := json.Marshal(nitfc)
		if err == nil {
			schema.Example = json.RawMessage(example)
		}
	}
	if jtype, ok := nitfc.(interface {
		ValuesJSON() []json.RawMessage
	}); ok {
		schema.Enum = jtype.ValuesJSON()
	} else if rtype.Implements(reflect.TypeFor[encoding.TextMarshaler]()) && !rtype.Implements(reflect.TypeFor[json.Marshaler]()) {
		if rtype == reflect.TypeOf(netip.Addr{}) {
			schema.AnyOf = []*oas.Schema{
				{
					Type:   []oas.Type{oas.Types.String},
					Format: &oas.Formats.IPv4,
				},
				{
					Type:   []oas.Type{oas.Types.String},
					Format: &oas.Formats.IPv6,
				},
			}
		} else {
			schema.Type = []oas.Type{oas.Types.String}
		}
	} else {
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
			schema.Format = &oas.Formats.Int32
		case reflect.Int64:
			schema.Type = []oas.Type{oas.Types.Integer}
			schema.Format = &oas.Formats.Int64
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
		case reflect.Float32:
			schema.Type = []oas.Type{oas.Types.Number}
			schema.Format = &oas.Formats.Float
		case reflect.Float64:
			schema.Type = []oas.Type{oas.Types.Number}
			schema.Format = &oas.Formats.Double
		case reflect.String:
			if rtype == reflect.TypeOf(email.Address("")) {
				schema.Format = &oas.Formats.Email
			}
			schema.Type = []oas.Type{oas.Types.String}
		case reflect.Map:
			schema.Type = []oas.Type{oas.Types.Object}
			schema.PropertyNames = schemaFor(reg, rtype.Key())
			schema.AdditionalProperties = schemaFor(reg, rtype.Elem())
		case reflect.Pointer:
			return schemaFor(reg, rtype.Elem())
		case reflect.Slice, reflect.Array:
			schema.Type = []oas.Type{oas.Types.Array}
			schema.Items = schemaFor(reg, rtype.Elem())
			if rtype.Kind() == reflect.Array {
				schema.MaxItems = rtype.Len()
			}
		case reflect.Chan:
			return schemaFor(reg, rtype.Elem())
		case reflect.Func:
			if isSeq, isSeq2 := isIteratorType(rtype); isSeq || isSeq2 {
				schema.Type = []oas.Type{oas.Types.Array}
				if isSeq2 {
					yieldType := rtype.In(0)
					keyType := yieldType.In(0)
					valueType := yieldType.In(1)
					pairSchema := &oas.Schema{
						Type: []oas.Type{oas.Types.Array},
						Items: &oas.Schema{
							OneOf: []*oas.Schema{
								schemaFor(reg, keyType),
								schemaFor(reg, valueType),
							},
						},
					}
					schema.Items = pairSchema
				} else {
					yieldType := rtype.In(0)
					elementType := yieldType.In(0)
					schema.Items = schemaFor(reg, elementType)
				}
				return schema
			}
		case reflect.Struct:
			if rtype == reflect.TypeOf(time.Time{}) {
				schema.Type = []oas.Type{oas.Types.String}
				schema.Format = &oas.Formats.DateTime
			} else {
				schema.Type = []oas.Type{oas.Types.Object}
				schema.Properties = make(map[oas.PropertyName]*oas.Schema)
				addFieldsToSchema(schema, reg, rtype)
			}
		}
		min, isType := rtype.MethodByName("Min")
		if isType {
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
		max, isType := rtype.MethodByName("Max")
		if isType {
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
	}
	if useRef {
		if existing := reg.Lookup(namespace, name); existing != nil {
			existing.Format = formatFor(rtype)
			return existing
		}
	}
	return schema
}
