// Package oas provides a representation of the OpenAPI Specification (OAS) Version 3.2.0
package oas

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"

	"github.com/iqhive/runtime.link/xyz"
)

type (
	Email       string
	URI         string
	URL         string
	ContentType string
	Name        string
	Readable    string
	Markdown    string
)

type Version string

type Document struct {
	OpenAPI               Version                       `json:"openapi"`
	Information           Information                   `json:"info"`
	SchemaDialect         URI                           `json:"jsonSchemaDialect,omitempty"`
	Servers               []Server                      `json:"servers,omitempty"`
	Paths                 map[string]PathItem           `json:"paths,omitempty"`
	Webhooks              map[string]*PathItem          `json:"webhooks,omitempty"`
	Components            *Components                   `json:"components,omitempty"`
	Security              []map[string]SecuritySchemeID `json:"security,omitempty"`
	Tags                  []Tag                         `json:"tags,omitempty"`
	ExternalDocumentation *ExternalDocumentation        `json:"externalDocs,omitempty"`

	// namespaces is the namespace the schemas of each Go package are in, by
	// the path of the package, and schemas is where that of each registered
	// type is.
	namespaces map[string]string
	schemas    map[reflect.Type]component
}

// component is where in a [Document] a schema is.
type component struct {
	namespace, name string
}

// Information about an API.
type Information struct {
	Title           Readable        `json:"title"`
	Summary         Readable        `json:"summary,omitempty"`
	Version         Version         `json:"version"`
	License         *Licensing      `json:"license,omitempty"`
	TermsConditions URL             `json:"termsOfService,omitempty"`
	Description     Markdown        `json:"description,omitempty"`
	Contact         *ContactDetails `json:"contact,omitempty"`
}

type LicenseID string

// Licensing information for the implementation of the API.
type Licensing struct {
	ID   LicenseID `json:"identifier,omitempty"`
	Name Readable  `json:"name"`
	URL  URL       `json:"url,omitempty"`
}

// ContactDetails for an API.
type ContactDetails struct {
	Name  Name  `json:"name,omitempty"`
	URL   URL   `json:"url,omitempty"`
	Email Email `json:"email,omitempty"`
}

type Reference struct {
	URI         URI      `json:"$ref"`
	Summary     Readable `json:"summary,omitempty"`
	Description Readable `json:"description,omitempty"`
}

type Server struct {
	URL         URL                       `json:"url"`
	Description Readable                  `json:"description,omitempty"`
	Variables   map[string]ServerVariable `json:"variables,omitempty"`
}

type ServerVariable struct {
	Enum        []string `json:"enum,omitempty"`
	Default     string   `json:"default"`
	Description Readable `json:"description,omitempty"`
}

type SecuritySchemeID string

type Components struct {
	Schemas         map[string]*Schema                   `json:"schemas,omitempty"`
	Responses       map[string]*Response                 `json:"responses,omitempty"`
	Parameters      map[string]*Parameter                `json:"parameters,omitempty"`
	Examples        map[string]*Example                  `json:"examples,omitempty"`
	RequestBodies   map[string]*RequestBody              `json:"requestBodies,omitempty"`
	Headers         map[string]*Header                   `json:"headers,omitempty"`
	SecuritySchemes map[SecuritySchemeID]*SecurityScheme `json:"securitySchemes,omitempty"`
	Links           map[string]*Link                     `json:"links,omitempty"`
	Callbacks       map[string]*Callback                 `json:"callbacks,omitempty"`
	PathItems       map[string]*PathItem                 `json:"pathItems,omitempty"`
}

type PathItem struct {
	Extends    *PathItem    `json:"$ref,omitempty"`
	Summary    Readable     `json:"summary,omitempty"`
	Desciption Readable     `json:"description,omitempty"`
	Get        *Operation   `json:"get,omitempty"`
	Put        *Operation   `json:"put,omitempty"`
	Post       *Operation   `json:"post,omitempty"`
	Delete     *Operation   `json:"delete,omitempty"`
	Options    *Operation   `json:"options,omitempty"`
	Head       *Operation   `json:"head,omitempty"`
	Patch      *Operation   `json:"patch,omitempty"`
	Trace      *Operation   `json:"trace,omitempty"`
	Query      *Operation   `json:"query,omitempty"`
	Servers    []Server     `json:"servers,omitempty"`
	Parameters []*Parameter `json:"parameters,omitempty"`

	// AdditionalOperations is the operations of the methods that have no
	// field above, by method, written as it is sent.
	AdditionalOperations map[string]*Operation `json:"additionalOperations,omitempty"`
}

type OperationID string

type Operation struct {
	ID          OperationID                   `json:"operationId,omitempty"`
	Tags        []string                      `json:"tags,omitempty"`
	Summary     Readable                      `json:"summary,omitempty"`
	Description Readable                      `json:"description,omitempty"`
	SeeAlso     *ExternalDocumentation        `json:"externalDocs,omitempty"`
	Parameters  []*Parameter                  `json:"parameters,omitempty"`
	RequestBody *RequestBody                  `json:"requestBody,omitempty"`
	Responses   map[ResponseKey]*Response     `json:"responses,omitempty"`
	Callbacks   map[string]*Callback          `json:"callbacks,omitempty"`
	Deprecated  bool                          `json:"deprecated,omitempty"`
	Security    []map[string]SecuritySchemeID `json:"security,omitempty"`
	Servers     []Server                      `json:"servers,omitempty"`
}

type ExternalDocumentation struct {
	Description Readable `json:"description,omitempty"`
	URL         URL      `json:"url"`
}

type Parameter struct {
	Name            Readable             `json:"name"`
	In              ParameterLocation    `json:"in"`
	Description     Readable             `json:"description,omitempty"`
	Required        bool                 `json:"required,omitempty"`
	Deprecated      bool                 `json:"deprecated,omitempty"`
	AllowEmptyValue bool                 `json:"allowEmptyValue,omitempty"`
	Style           ParameterStyle       `json:"style,omitempty"`
	Explode         bool                 `json:"explode,omitempty"`
	AllowReserved   bool                 `json:"allowReserved,omitempty"`
	Schema          *Schema              `json:"schema,omitempty"`
	Example         json.RawMessage      `json:"example,omitempty"`
	Examples        map[string]*Example  `json:"examples,omitempty"`
	Content         map[string]MediaType `json:"content,omitempty"`
}

type ParameterLocation xyz.Switch[string, struct {
	Query  ParameterLocation `json:"query"`
	Header ParameterLocation `json:"header"`
	Path   ParameterLocation `json:"path"`
	Cookie ParameterLocation `json:"cookie"`
}]

var ParameterLocations = xyz.AccessorFor(ParameterLocation.Values)

type ParameterStyle xyz.Switch[string, struct {
	Matrix         ParameterStyle `json:"matrix"`
	Label          ParameterStyle `json:"label"`
	Form           ParameterStyle `json:"form"`
	Simple         ParameterStyle `json:"simple"`
	SpaceDelimited ParameterStyle `json:"spaceDelimited"`
	PipeDelimited  ParameterStyle `json:"pipeDelimited"`
	DeepObject     ParameterStyle `json:"deepObject"`
}]

var ParameterStyles = xyz.AccessorFor(ParameterStyle.Values)

type RequestBody struct {
	Description Readable                  `json:"description,omitempty"`
	Content     map[ContentType]MediaType `json:"content"`
	Required    bool                      `json:"required,omitempty"`
}

type MediaType struct {
	Schema   *Schema             `json:"schema,omitempty"`
	Example  json.RawMessage     `json:"example,omitempty"`
	Examples map[string]*Example `json:"examples,omitempty"`
	Encoding map[string]Encoding `json:"encoding,omitempty"`
}

type ResponseKey xyz.Switch[string, struct {
	Default ResponseKey `json:"default"`
}]

var ResponseKeys = xyz.AccessorFor(ResponseKey.Values)

type Encoding struct {
	ContentType   ContentType        `json:"contentType,omitempty"`
	Headers       map[string]*Header `json:"headers,omitempty"`
	Style         ParameterStyle     `json:"style,omitempty"`
	Explode       bool               `json:"explode,omitempty"`
	AllowReserved bool               `json:"allowReserved,omitempty"`
}

type Response struct {
	Description Readable                  `json:"description"`
	Headers     map[string]*Header        `json:"headers,omitempty"`
	Content     map[ContentType]MediaType `json:"content,omitempty"`
	Links       map[string]*Link          `json:"links,omitempty"`
}

type Callback map[string]*PathItem

type Example struct {
	Summary       Readable        `json:"summary,omitempty"`
	Description   Readable        `json:"description,omitempty"`
	Value         json.RawMessage `json:"value,omitempty"`
	ExternalValue URI             `json:"externalValue,omitempty"`
}

type Expression string

type Link struct {
	OperationRef *Operation     `json:"operationRef,omitempty"`
	OperationID  OperationID    `json:"operationId,omitempty"`
	Parameters   map[string]any `json:"parameters,omitempty"`
	RequestBody  any            `json:"requestBody,omitempty"`
	Description  Readable       `json:"description,omitempty"`
	Server       *Server        `json:"server,omitempty"`
}

type Header struct {
	Description     Readable             `json:"description,omitempty"`
	Required        bool                 `json:"required,omitempty"`
	Deprecated      bool                 `json:"deprecated,omitempty"`
	AllowEmptyValue bool                 `json:"allowEmptyValue,omitempty"`
	Style           ParameterStyle       `json:"style,omitempty"`
	Explode         bool                 `json:"explode,omitempty"`
	AllowReserved   bool                 `json:"allowReserved,omitempty"`
	Schema          *Schema              `json:"schema,omitempty"`
	Example         json.RawMessage      `json:"example,omitempty"`
	Examples        map[string]*Example  `json:"examples,omitempty"`
	Content         map[string]MediaType `json:"content,omitempty"`
}

type Tag struct {
	Name                  Readable              `json:"name"`
	Description           Markdown              `json:"description,omitempty"`
	ExternalDocumentation ExternalDocumentation `json:"externalDocs,omitempty"`
}

type Discriminator struct {
	PropertyName string            `json:"propertyName"`
	Mapping      map[string]string `json:"mapping,omitempty"`
}

type XML struct {
	Name      string `json:"name,omitempty"`
	Namespace URI    `json:"namespace,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	Attribute bool   `json:"attribute,omitempty"`
	Wrapped   bool   `json:"wrapped,omitempty"`
}

type SecurityScheme struct {
	Type         SecuritySchemeType `json:"type"`
	Description  Markdown           `json:"description,omitempty"`
	Name         string             `json:"name,omitempty"`
	In           ParameterLocation  `json:"in,omitempty"`
	Scheme       string             `json:"scheme,omitempty"`
	BearerFormat string             `json:"bearerFormat,omitempty"`
	Flows        *OauthFlows        `json:"flows,omitempty"`
	ConnectURL   URL                `json:"openIdConnectUrl,omitempty"`
}

type SecuritySchemeType xyz.Switch[string, struct {
	Key       SecuritySchemeType `json:"apiKey"`
	HTTP      SecuritySchemeType `json:"http"`
	MutualTLS SecuritySchemeType `json:"mutualTLS"`
	OAuth2    SecuritySchemeType `json:"oauth2"`
	OpenID    SecuritySchemeType `json:"openIdConnect"`
}]

var SecuritySchemeTypes = xyz.AccessorFor(SecuritySchemeType.Values)

type OauthFlows struct {
	Implicit          *OauthFlow `json:"implicit,omitempty"`
	Password          *OauthFlow `json:"password,omitempty"`
	ClientCredentials *OauthFlow `json:"clientCredentials,omitempty"`
	AuthorizationCode *OauthFlow `json:"authorizationCode,omitempty"`
}

type OauthFlow struct {
	Authorization URL            `json:"authorizationUrl,omitempty"`
	Token         URL            `json:"tokenUrl,omitempty"`
	Refresh       URL            `json:"refreshUrl,omitempty"`
	Scopes        map[string]any `json:"scopes,omitempty"`
}

type PropertyName string

type TypeSet []Type

func (t TypeSet) MarshalJSON() ([]byte, error) {
	if len(t) == 1 {
		return json.Marshal(t[0])
	}
	return json.Marshal([]Type(t))
}

func (t *TypeSet) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if data[0] == '"' {
		var s Type
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*t = TypeSet{s}
		return nil
	}
	return json.Unmarshal(data, (*[]Type)(t))
}

// Schema based on https://json-schema.org/draft/2020-12/json-schema-core
type Schema struct {
	ID  URI `json:"$id,omitempty"`
	Ref URI `json:"$ref,omitempty"`

	Defs map[string]*Schema `json:"$defs,omitempty"`

	Dialect URI    `json:"$schema,omitempty"`
	Anchor  string `json:"$anchor,omitempty"`

	AllOf []*Schema `json:"allOf,omitempty"`
	AnyOf []*Schema `json:"anyOf,omitempty"`
	OneOf []*Schema `json:"oneOf,omitempty"`
	Not   *Schema   `json:"not,omitempty"`

	Type TypeSet `json:"type,omitempty"`

	Title             Readable                        `json:"title,omitempty"`
	Description       Readable                        `json:"description,omitempty"`
	Properties        map[PropertyName]*Schema        `json:"properties,omitempty"`
	Required          []PropertyName                  `json:"required,omitempty"`
	DependentRequired map[PropertyName][]PropertyName `json:"dependentRequired,omitempty"`

	DependentSchemas map[PropertyName]*Schema `json:"dependentSchemas,omitempty"`

	If   *Schema `json:"if,omitempty"`
	Then *Schema `json:"then,omitempty"`
	Else *Schema `json:"else,omitempty"`

	MinLength  int     `json:"minLength,omitempty"`
	MaxLength  int     `json:"maxLength,omitempty"`
	MultipleOf float64 `json:"multipleOf,omitempty"`

	Default json.RawMessage `json:"default,omitempty"`

	Minimum *float64 `json:"minimum,omitempty"`
	Maximum *float64 `json:"maximum,omitempty"`

	ExclusiveMinimum float64 `json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum float64 `json:"exclusiveMaximum,omitempty"`

	Const       json.RawMessage   `json:"const,omitempty"`
	Enum        []json.RawMessage `json:"enum,omitempty"`
	PrefixItems []*Schema         `json:"prefixItems,omitempty"`

	Contains    *Schema `json:"contains,omitempty"`
	MinContains int     `json:"minContains,omitempty"`
	MaxContains int     `json:"maxContains,omitempty"`

	MinItems    int  `json:"minItems,omitempty"`
	MaxItems    int  `json:"maxItems,omitempty"`
	UniqueItems bool `json:"uniqueItems,omitempty"`

	Pattern           string             `json:"pattern,omitempty"`
	PatternProperties map[string]*Schema `json:"patternProperties,omitempty"`
	PropertyNames     *Schema            `json:"propertyNames,omitempty"`

	MinProperties int `json:"minProperties,omitempty"`
	MaxProperties int `json:"maxProperties,omitempty"`

	Items *Schema `json:"items,omitempty"`

	Format *Format `json:"format,omitempty"`

	AdditionalProperties *Schema `json:"additionalProperties,omitempty"`

	ReadOnly  bool `json:"readOnly,omitempty"`
	WriteOnly bool `json:"writeOnly,omitempty"`

	Deprecated bool `json:"deprecated,omitempty"`

	ContentMediaType ContentType `json:"contentMediaType,omitempty"`
	ContentEncoding  string      `json:"contentEncoding,omitempty"`

	// OpenAPI extensions below

	Discriminator         *Discriminator         `json:"discriminator,omitempty"`
	XML                   *XML                   `json:"xml,omitempty"`
	ExternalDocumentation *ExternalDocumentation `json:"externalDocs,omitempty"`
	Example               json.RawMessage        `json:"example,omitempty"`
	Examples              []json.RawMessage      `json:"examples,omitempty"`

	// defs is the name in Defs of each type registered with this schema.
	defs map[reflect.Type]string
}

type Property struct {
	Type        Type     `json:"type,omitempty"`
	Description Readable `json:"description,omitempty"`
}

type Type xyz.Switch[string, struct {
	String  Type `json:"string"`
	Number  Type `json:"number"`
	Integer Type `json:"integer"`
	Object  Type `json:"object"`
	Array   Type `json:"array"`
	Bool    Type `json:"boolean"`
	Null    Type `json:"null"`
}]

var Types = xyz.AccessorFor(Type.Values)

type Format xyz.Switch[string, struct {
	DateTime Format `json:"date-time"`
	Time     Format `json:"time"`
	Date     Format `json:"date"`
	Duration Format `json:"duration"`
	Email    Format `json:"email"`
	Hostname Format `json:"hostname"`
	IPv4     Format `json:"ipv4"`
	IPv6     Format `json:"ipv6"`
	UUID     Format `json:"uuid"`
	URI      Format `json:"uri"`
	Regex    Format `json:"regex"`

	Int32    Format `json:"int32"`
	Int64    Format `json:"int64"`
	Float    Format `json:"float"`
	Double   Format `json:"double"`
	Password Format `json:"password"`
}]

var Formats = xyz.AccessorFor(Format.Values)

// Registry holds the schemas of Go types for others to refer to. A schema is
// kept by its type, not by its name: two packages may each have a type of one
// name, and paths that end alike, and theirs are still two schemas.
type Registry interface {
	// Lookup returns a reference to the schema registered for a type, or
	// nil when there is none.
	Lookup(reflect.Type) *Schema

	// Register adds the schema of a type, under the name given unless a
	// type registered before it has that one, when more of the path of the
	// type's package is put in front of it.
	Register(reflect.Type, string, *Schema)
}

func (doc *Document) Lookup(rtype reflect.Type) *Schema {
	at, ok := doc.schemas[rtype]
	if !ok {
		return nil
	}
	return &Schema{Ref: URI("#/components/schemas/" + at.namespace + "/$defs/" + at.name)}
}

func (doc *Document) Register(rtype reflect.Type, name string, schema *Schema) {
	if doc.Components == nil {
		doc.Components = &Components{}
	}
	if doc.Components.Schemas == nil {
		doc.Components.Schemas = make(map[string]*Schema)
	}
	if doc.schemas == nil {
		doc.schemas = make(map[reflect.Type]component)
	}
	at, ok := doc.schemas[rtype]
	if !ok {
		at.namespace = doc.namespaceOf(rtype.PkgPath())
	}
	pkg := doc.Components.Schemas[at.namespace]
	if pkg == nil {
		pkg = &Schema{}
	}
	if pkg.Defs == nil {
		pkg.Defs = make(map[string]*Schema)
	}
	if !ok {
		// Two types of one package only share a name when it was cut
		// short, as the type arguments of a generic type are.
		at.name = vacant([]string{name}, func(name string) bool { return pkg.Defs[name] != nil })
	}
	pkg.Defs[at.name] = schema
	doc.Components.Schemas[at.namespace] = pkg
	doc.schemas[rtype] = at
}

// namespaceOf returns the namespace for the schemas of a package, which is the
// last element of its path unless another package has that one already.
func (doc *Document) namespaceOf(pkg string) string {
	if namespace, ok := doc.namespaces[pkg]; ok {
		return namespace
	}
	if doc.namespaces == nil {
		doc.namespaces = make(map[string]string)
	}
	namespace := vacant(qualifiers(pkg), func(namespace string) bool {
		_, isTaken := doc.Components.Schemas[namespace]
		return isTaken
	})
	doc.namespaces[pkg] = namespace
	return namespace
}

// qualifiers returns the names a package may go by, the shortest first: the
// last element of its path, then that with the one before it, and so on. They
// are joined by dots, as a slash would end the name in a reference.
func qualifiers(pkg string) []string {
	elements := strings.Split(pkg, "/")
	names := make([]string, 0, len(elements))
	for i := len(elements) - 1; i >= 0; i-- {
		names = append(names, strings.Join(elements[i:], "."))
	}
	return names
}

// vacant returns the first of the preferred names that is not taken, and when
// they all are, the first of them with the lowest number after it that is not.
func vacant(preferred []string, isTaken func(string) bool) string {
	for _, name := range preferred {
		if !isTaken(name) {
			return name
		}
	}
	for n := 2; ; n++ {
		if name := preferred[0] + "_" + strconv.Itoa(n); !isTaken(name) {
			return name
		}
	}
}

func (schema *Schema) Lookup(rtype reflect.Type) *Schema {
	name, ok := schema.defs[rtype]
	if !ok || schema.Defs[name] == nil {
		return nil
	}
	return &Schema{Ref: URI("#/$defs/" + name), Type: schema.Defs[name].Type}
}

func (schema *Schema) Register(rtype reflect.Type, name string, value *Schema) {
	if schema.Defs == nil {
		schema.Defs = make(map[string]*Schema)
	}
	if schema.defs == nil {
		schema.defs = make(map[reflect.Type]string)
	}
	at, ok := schema.defs[rtype]
	if !ok {
		preferred := []string{name}
		for _, qualifier := range qualifiers(rtype.PkgPath()) {
			preferred = append(preferred, qualifier+"."+name)
		}
		at = vacant(preferred, func(name string) bool { return schema.Defs[name] != nil })
		schema.defs[rtype] = at
	}
	schema.Defs[at] = value
}
