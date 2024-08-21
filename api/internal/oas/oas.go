// Package oas provides a representation of the OpenAPI Specification (OAS) Version 3.1.0
package oas

import (
	"encoding/json"

	"runtime.link/xyz"
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
	Servers    []Server     `json:"servers,omitempty"`
	Parameters []*Parameter `json:"parameters,omitempty"`
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

type ParameterLocation xyz.Tagged[string, struct {
	Query  ParameterLocation `txt:"query"`
	Header ParameterLocation `txt:"header"`
	Path   ParameterLocation `txt:"path"`
	Cookie ParameterLocation `txt:"cookie"`
}]

var ParameterLocations = xyz.AccessorFor(ParameterLocation.Values)

type ParameterStyle xyz.Tagged[string, struct {
	Matrix         ParameterStyle `txt:"matrix"`
	Label          ParameterStyle `txt:"label"`
	Form           ParameterStyle `txt:"form"`
	Simple         ParameterStyle `txt:"simple"`
	SpaceDelimited ParameterStyle `txt:"spaceDelimited"`
	PipeDelimited  ParameterStyle `txt:"pipeDelimited"`
	DeepObject     ParameterStyle `txt:"deepObject"`
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

type ResponseKey xyz.Tagged[string, struct {
	Default ResponseKey `txt:"default"`
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

type SecuritySchemeType xyz.Tagged[string, struct {
	Key       SecuritySchemeType `txt:"apiKey"`
	HTTP      SecuritySchemeType `txt:"http"`
	MutualTLS SecuritySchemeType `txt:"mutualTLS"`
	OAuth2    SecuritySchemeType `txt:"oauth2"`
	OpenID    SecuritySchemeType `txt:"openIdConnect"`
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
}

type Property struct {
	Type        Type     `json:"type,omitempty"`
	Description Readable `json:"description,omitempty"`
}

type Type xyz.Tagged[string, struct {
	String  Type `txt:"string"`
	Number  Type `txt:"number"`
	Integer Type `txt:"integer"`
	Object  Type `txt:"object"`
	Array   Type `txt:"array"`
	Bool    Type `txt:"boolean"`
	Null    Type `txt:"null"`
}]

var Types = xyz.AccessorFor(Type.Values)

type Format xyz.Tagged[string, struct {
	DateTime Format `txt:"date-time"`
	Time     Format `txt:"time"`
	Date     Format `txt:"date"`
	Duration Format `txt:"duration"`
	Email    Format `txt:"email"`
	Hostname Format `txt:"hostname"`
	IPv4     Format `txt:"ipv4"`
	IPv6     Format `txt:"ipv6"`
	UUID     Format `txt:"uuid"`
	URI      Format `txt:"uri"`
	Regex    Format `txt:"regex"`
}]

var Formats = xyz.AccessorFor(Format.Values)

type Registry interface {
	Lookup(string, string) *Schema
	Register(string, string, *Schema)
}

func (doc *Document) Lookup(namespace, name string) *Schema {
	if doc.Components == nil {
		return nil
	}
	pkg, ok := doc.Components.Schemas[namespace]
	if !ok {
		return nil
	}
	schema := pkg.Defs[name]
	if schema == nil {
		return nil
	}
	return &Schema{Ref: URI("#/components/schemas/" + namespace + "/$defs/" + name)}
}

func (doc *Document) Register(namespace, name string, schema *Schema) {
	if doc.Components == nil {
		doc.Components = &Components{}
	}
	if doc.Components.Schemas == nil {
		doc.Components.Schemas = make(map[string]*Schema)
	}
	pkg, ok := doc.Components.Schemas[namespace]
	if !ok {
		pkg = &Schema{}
	}
	if pkg.Defs == nil {
		pkg.Defs = make(map[string]*Schema)
	}
	pkg.Defs[name] = schema
	doc.Components.Schemas[namespace] = pkg
}

func (schema *Schema) Lookup(namespace, name string) *Schema {
	if schema.Defs == nil {
		return nil
	}
	schema = schema.Defs[name]
	if schema == nil {
		return nil
	}
	return &Schema{Ref: URI("#/$defs/" + name), Type: schema.Type}
}

func (schema *Schema) Register(namespace, name string, value *Schema) {
	if schema.Defs == nil {
		schema.Defs = make(map[string]*Schema)
	}
	schema.Defs[name] = value
}
