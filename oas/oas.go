// Package oas provides a representation of the OpenAPI Specification (OAS) Version 3.1.0
package oas

import (
	"encoding/json"

	"runtime.link/ref/oss/license"
	"runtime.link/ref/std/email"
	"runtime.link/ref/std/media"
	"runtime.link/ref/std/uri"
	"runtime.link/ref/std/url"
	"runtime.link/txt/std/human"
	"runtime.link/txt/std/markdown"
	"runtime.link/xyz"
)

type Version string

type Document struct {
	OpenAPI               Version                       `json:"openapi"`
	Information           Information                   `json:"info"`
	SchemaDialect         uri.String                    `json:"jsonSchemaDialect,omitempty"`
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
	Title           human.Readable  `json:"title"`
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

type Reference struct {
	URI         uri.String     `json:"$ref"`
	Summary     human.Readable `json:"summary,omitempty"`
	Description human.Readable `json:"description,omitempty"`
}

type Server struct {
	URL         url.String                `json:"url"`
	Description human.Readable            `json:"description,omitempty"`
	Variables   map[string]ServerVariable `json:"variables,omitempty"`
}

type ServerVariable struct {
	Enum        []string       `json:"enum,omitempty"`
	Default     string         `json:"default"`
	Description human.Readable `json:"description,omitempty"`
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
	Extends    *PathItem      `json:"$ref,omitempty"`
	Summary    human.Readable `json:"summary,omitempty"`
	Desciption human.Readable `json:"description,omitempty"`
	Get        *Operation     `json:"get,omitempty"`
	Put        *Operation     `json:"put,omitempty"`
	Post       *Operation     `json:"post,omitempty"`
	Delete     *Operation     `json:"delete,omitempty"`
	Options    *Operation     `json:"options,omitempty"`
	Head       *Operation     `json:"head,omitempty"`
	Patch      *Operation     `json:"patch,omitempty"`
	Trace      *Operation     `json:"trace,omitempty"`
	Servers    []Server       `json:"servers,omitempty"`
	Parameters []*Parameter   `json:"parameters,omitempty"`
}

type OperationID string

type Operation struct {
	ID          OperationID                   `json:"operationId,omitempty"`
	Tags        []string                      `json:"tags,omitempty"`
	Summary     human.Readable                `json:"summary,omitempty"`
	Description human.Readable                `json:"description,omitempty"`
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
	Description human.Readable `json:"description,omitempty"`
	URL         url.String     `json:"url"`
}

type Parameter struct {
	Name            human.Readable       `json:"name"`
	In              ParameterLocation    `json:"in"`
	Description     human.Readable       `json:"description,omitempty"`
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
	Query  ParameterLocation `txt:"query"`
	Header ParameterLocation `txt:"header"`
	Path   ParameterLocation `txt:"path"`
	Cookie ParameterLocation `txt:"cookie"`
}]

var ParameterLocations = xyz.AccessorFor(ParameterLocation.Values)

type ParameterStyle xyz.Switch[string, struct {
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
	Description human.Readable           `json:"description,omitempty"`
	Content     map[media.Type]MediaType `json:"content"`
	Required    bool                     `json:"required,omitempty"`
}

type MediaType struct {
	Schema   *Schema             `json:"schema,omitempty"`
	Example  json.RawMessage     `json:"example,omitempty"`
	Examples map[string]*Example `json:"examples,omitempty"`
	Encoding map[string]Encoding `json:"encoding,omitempty"`
}

type ResponseKey xyz.Switch[string, struct {
	Default ResponseKey `txt:"default"`
}]

type Encoding struct {
	ContentType   media.Type         `json:"contentType,omitempty"`
	Headers       map[string]*Header `json:"headers,omitempty"`
	Style         ParameterStyle     `json:"style,omitempty"`
	Explode       bool               `json:"explode,omitempty"`
	AllowReserved bool               `json:"allowReserved,omitempty"`
}

type Response struct {
	Description human.Readable           `json:"description"`
	Headers     map[string]*Header       `json:"headers,omitempty"`
	Content     map[media.Type]MediaType `json:"content,omitempty"`
	Links       map[string]*Link         `json:"links,omitempty"`
}

type Callback map[string]*PathItem

type Example struct {
	Summary       human.Readable  `json:"summary,omitempty"`
	Description   human.Readable  `json:"description,omitempty"`
	Value         json.RawMessage `json:"value,omitempty"`
	ExternalValue uri.String      `json:"externalValue,omitempty"`
}

type Expression string

type Link struct {
	OperationRef *Operation     `json:"operationRef,omitempty"`
	OperationID  OperationID    `json:"operationId,omitempty"`
	Parameters   map[string]any `json:"parameters,omitempty"`
	RequestBody  any            `json:"requestBody,omitempty"`
	Description  human.Readable `json:"description,omitempty"`
	Server       *Server        `json:"server,omitempty"`
}

type Header struct {
	Description     human.Readable       `json:"description,omitempty"`
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
	Name                  human.Readable        `json:"name"`
	Description           markdown.String       `json:"description,omitempty"`
	ExternalDocumentation ExternalDocumentation `json:"externalDocs,omitempty"`
}

type Discriminator struct {
	PropertyName string            `json:"propertyName"`
	Mapping      map[string]string `json:"mapping,omitempty"`
}

type XML struct {
	Name      string     `json:"name,omitempty"`
	Namespace uri.String `json:"namespace,omitempty"`
	Prefix    string     `json:"prefix,omitempty"`
	Attribute bool       `json:"attribute,omitempty"`
	Wrapped   bool       `json:"wrapped,omitempty"`
}

type SecurityScheme struct {
	Type         SecuritySchemeType `json:"type"`
	Description  markdown.String    `json:"description,omitempty"`
	Name         string             `json:"name,omitempty"`
	In           ParameterLocation  `json:"in,omitempty"`
	Scheme       string             `json:"scheme,omitempty"`
	BearerFormat string             `json:"bearerFormat,omitempty"`
	Flows        *OauthFlows        `json:"flows,omitempty"`
	ConnectURL   url.String         `json:"openIdConnectUrl,omitempty"`
}

type SecuritySchemeType xyz.Switch[string, struct {
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
	Authorization url.String     `json:"authorizationUrl,omitempty"`
	Token         url.String     `json:"tokenUrl,omitempty"`
	Refresh       url.String     `json:"refreshUrl,omitempty"`
	Scopes        map[string]any `json:"scopes,omitempty"`
}

type PropertyName string

// Schema based on https://json-schema.org/draft/2020-12/json-schema-core
type Schema struct {
	ID uri.String `json:"$id,omitempty"`

	Defs map[string]*Schema `json:"$defs,omitempty"`

	Dialect uri.String `json:"$schema,omitempty"`
	Anchor  string     `json:"$anchor,omitempty"`

	AllOf []*Schema `json:"allOf,omitempty"`
	AnyOf []*Schema `json:"anyOf,omitempty"`
	OneOf []*Schema `json:"oneOf,omitempty"`
	Not   *Schema   `json:"not,omitempty"`

	Type []Type `json:"type,omitempty"`

	Title             human.Readable                  `json:"title,omitempty"`
	Description       human.Readable                  `json:"description,omitempty"`
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

	ContentMediaType media.Type `json:"contentMediaType,omitempty"`
	ContentEncoding  string     `json:"contentEncoding,omitempty"`

	// OpenAPI extensions below

	Discriminator         *Discriminator         `json:"discriminator,omitempty"`
	XML                   *XML                   `json:"xml,omitempty"`
	ExternalDocumentation *ExternalDocumentation `json:"externalDocs,omitempty"`
	Example               json.RawMessage        `json:"example,omitempty"`
	Examples              []json.RawMessage      `json:"examples,omitempty"`
}

type Property struct {
	Type        Type           `json:"type,omitempty"`
	Description human.Readable `json:"description,omitempty"`
}

type Type xyz.Switch[string, struct {
	String  Type `txt:"string"`
	Number  Type `txt:"number"`
	Integer Type `txt:"integer"`
	Object  Type `txt:"object"`
	Array   Type `txt:"array"`
	Bool    Type `txt:"boolean"`
	Null    Type `txt:"null"`
}]

var Types = xyz.AccessorFor(Type.Values)

type Format xyz.Switch[string, struct {
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
