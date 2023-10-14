package api

import (
	"encoding/json"

	"runtime.link/ref/std/media"
	"runtime.link/ref/std/uri"
	"runtime.link/ref/std/url"
	"runtime.link/txt/std/human"
	"runtime.link/txt/std/markdown"
	"runtime.link/xyz"
)

type oasReference struct {
	URI         uri.String     `json:"$ref"`
	Summary     human.Readable `json:"summary,omitempty"`
	Description human.Readable `json:"description,omitempty"`
}

type oasServer struct {
	URL         url.String                   `json:"url"`
	Description human.Readable               `json:"description,omitempty"`
	Variables   map[string]oasServerVariable `json:"variables,omitempty"`
}

type oasServerVariable struct {
	Enum        []string       `json:"enum,omitempty"`
	Default     string         `json:"default"`
	Description human.Readable `json:"description,omitempty"`
}

type oasSecuritySchemeID string

type oasComponents struct {
	Schemas         map[string]*oasSchema                      `json:"schemas,omitempty"`
	Responses       map[string]*oasResponse                    `json:"responses,omitempty"`
	Parameters      map[string]*oasParameter                   `json:"parameters,omitempty"`
	Examples        map[string]*oasExample                     `json:"examples,omitempty"`
	RequestBodies   map[string]*oasRequestBody                 `json:"requestBodies,omitempty"`
	Headers         map[string]*oasHeader                      `json:"headers,omitempty"`
	SecuritySchemes map[oasSecuritySchemeID]*oasSecurityScheme `json:"securitySchemes,omitempty"`
	Links           map[string]*oasLink                        `json:"links,omitempty"`
	Callbacks       map[string]*oasCallback                    `json:"callbacks,omitempty"`
	Functions       map[string]*oasPathItem                    `json:"pathItems,omitempty"`
}

type oasPathItem struct {
	Extends    *oasPathItem    `json:"$ref,omitempty"`
	Summary    human.Readable  `json:"summary,omitempty"`
	Desciption human.Readable  `json:"description,omitempty"`
	Get        *oasFunction    `json:"get,omitempty"`
	Put        *oasFunction    `json:"put,omitempty"`
	Post       *oasFunction    `json:"post,omitempty"`
	Delete     *oasFunction    `json:"delete,omitempty"`
	Options    *oasFunction    `json:"options,omitempty"`
	Head       *oasFunction    `json:"head,omitempty"`
	Patch      *oasFunction    `json:"patch,omitempty"`
	Trace      *oasFunction    `json:"trace,omitempty"`
	Servers    []oasServer     `json:"servers,omitempty"`
	Parameters []*oasParameter `json:"parameters,omitempty"`
}

type oasFunctionID string

type oasFunction struct {
	ID          oasFunctionID                    `json:"operationId,omitempty"`
	Tags        []string                         `json:"tags,omitempty"`
	Summary     human.Readable                   `json:"summary,omitempty"`
	Description human.Readable                   `json:"description,omitempty"`
	SeeAlso     *oasExternalDocumentation        `json:"externalDocs,omitempty"`
	Parameters  []*oasParameter                  `json:"parameters,omitempty"`
	RequestBody *oasRequestBody                  `json:"requestBody,omitempty"`
	Responses   map[oasResponseKey]*oasResponse  `json:"responses,omitempty"`
	Callbacks   map[string]*oasCallback          `json:"callbacks,omitempty"`
	Deprecated  bool                             `json:"deprecated,omitempty"`
	Security    []map[string]oasSecuritySchemeID `json:"security,omitempty"`
	Servers     []oasServer                      `json:"servers,omitempty"`
}

type oasExternalDocumentation struct {
	Description human.Readable `json:"description,omitempty"`
	URL         url.String     `json:"url"`
}

type oasParameter struct {
	Name            human.Readable          `json:"name"`
	In              oasParameterLocation    `json:"in"`
	Description     human.Readable          `json:"description,omitempty"`
	Required        bool                    `json:"required,omitempty"`
	Deprecated      bool                    `json:"deprecated,omitempty"`
	AllowEmptyValue bool                    `json:"allowEmptyValue,omitempty"`
	Style           oasParameterStyle       `json:"style,omitempty"`
	Explode         bool                    `json:"explode,omitempty"`
	AllowReserved   bool                    `json:"allowReserved,omitempty"`
	Schema          *oasSchema              `json:"schema,omitempty"`
	Example         json.RawMessage         `json:"example,omitempty"`
	Examples        map[string]*oasExample  `json:"examples,omitempty"`
	Content         map[string]oasMediaType `json:"content,omitempty"`
}

type oasParameterLocation xyz.Switch[string, struct {
	Query  oasParameterLocation `txt:"query"`
	Header oasParameterLocation `txt:"header"`
	Path   oasParameterLocation `txt:"path"`
	Cookie oasParameterLocation `txt:"cookie"`
}]

type oasParameterStyle xyz.Switch[string, struct {
	Matrix         oasParameterStyle `txt:"matrix"`
	Label          oasParameterStyle `txt:"label"`
	Form           oasParameterStyle `txt:"form"`
	Simple         oasParameterStyle `txt:"simple"`
	SpaceDelimited oasParameterStyle `txt:"spaceDelimited"`
	PipeDelimited  oasParameterStyle `txt:"pipeDelimited"`
	DeepObject     oasParameterStyle `txt:"deepObject"`
}]

type oasRequestBody struct {
	Description human.Readable              `json:"description,omitempty"`
	Content     map[media.Type]oasMediaType `json:"content"`
	Required    bool                        `json:"required,omitempty"`
}

type oasMediaType struct {
	Schema   *oasSchema             `json:"schema,omitempty"`
	Example  json.RawMessage        `json:"example,omitempty"`
	Examples map[string]*oasExample `json:"examples,omitempty"`
	Encoding map[string]oasEncoding `json:"encoding,omitempty"`
}

type oasResponseKey xyz.Switch[string, struct {
	Default oasResponseKey `txt:"default"`
}]

type oasEncoding struct {
	ContentType   media.Type            `json:"contentType,omitempty"`
	Headers       map[string]*oasHeader `json:"headers,omitempty"`
	Style         oasParameterStyle     `json:"style,omitempty"`
	Explode       bool                  `json:"explode,omitempty"`
	AllowReserved bool                  `json:"allowReserved,omitempty"`
}

type oasResponse struct {
	Description human.Readable `json:"description"`
	Headers     map[string]*oasHeader
	Content     map[media.Type]oasMediaType `json:"content,omitempty"`
	Links       map[string]*oasLink         `json:"links,omitempty"`
}

type oasCallback map[string]*oasPathItem

type oasExample struct {
	Summary     human.Readable  `json:"summary,omitempty"`
	Description human.Readable  `json:"description,omitempty"`
	Value       json.RawMessage `json:"value,omitempty"`
	URI         uri.String      `json:"externalValue,omitempty"`
}

type oasExpression string

type oasLink struct {
	OperationRef *oasFunction   `json:"operationRef,omitempty"`
	OperationID  oasFunctionID  `json:"operationId,omitempty"`
	Parameters   map[string]any `json:"parameters,omitempty"`
	RequestBody  any            `json:"requestBody,omitempty"`
	Description  human.Readable `json:"description,omitempty"`
	Server       *oasServer     `json:"server,omitempty"`
}

type oasHeader struct {
	Description     human.Readable          `json:"description,omitempty"`
	Required        bool                    `json:"required,omitempty"`
	Deprecated      bool                    `json:"deprecated,omitempty"`
	AllowEmptyValue bool                    `json:"allowEmptyValue,omitempty"`
	Style           oasParameterStyle       `json:"style,omitempty"`
	Explode         bool                    `json:"explode,omitempty"`
	AllowReserved   bool                    `json:"allowReserved,omitempty"`
	Schema          *oasSchema              `json:"schema,omitempty"`
	Example         json.RawMessage         `json:"example,omitempty"`
	Examples        map[string]*oasExample  `json:"examples,omitempty"`
	Content         map[string]oasMediaType `json:"content,omitempty"`
}

type oasTag struct {
	Name        human.Readable           `json:"name"`
	Description markdown.String          `json:"description,omitempty"`
	SeeAlso     oasExternalDocumentation `json:"externalDocs,omitempty"`
}

type oasDiscriminator struct {
	PropertyName string            `json:"propertyName"`
	Mapping      map[string]string `json:"mapping,omitempty"`
}

type oasXML struct {
	Name      string     `json:"name,omitempty"`
	Namespace uri.String `json:"namespace,omitempty"`
	Prefix    string     `json:"prefix,omitempty"`
	Attribute bool       `json:"attribute,omitempty"`
	Wrapped   bool       `json:"wrapped,omitempty"`
}

type oasSecurityScheme struct {
	Type         oasSecuritySchemeType `json:"type"`
	Description  markdown.String       `json:"description,omitempty"`
	Name         string                `json:"name,omitempty"`
	In           oasParameterLocation  `json:"in,omitempty"`
	Scheme       string                `json:"scheme,omitempty"`
	BearerFormat string                `json:"bearerFormat,omitempty"`
	Oauth        *oasOauthFlows        `json:"flows,omitempty"`
	OpenID       url.String            `json:"openIdConnectUrl,omitempty"`
}

type oasSecuritySchemeType xyz.Switch[string, struct {
	Key       oasSecuritySchemeType `txt:"apiKey"`
	HTTP      oasSecuritySchemeType `txt:"http"`
	MutualTLS oasSecuritySchemeType `txt:"mutualTLS"`
	OAuth2    oasSecuritySchemeType `txt:"oauth2"`
	OpenID    oasSecuritySchemeType `txt:"openIdConnect"`
}]

type oasOauthFlows struct {
	Implicit          *oasOauthFlow `json:"implicit,omitempty"`
	Password          *oasOauthFlow `json:"password,omitempty"`
	ClientCredentials *oasOauthFlow `json:"clientCredentials,omitempty"`
	AuthorizationCode *oasOauthFlow `json:"authorizationCode,omitempty"`
}

type oasOauthFlow struct {
	Authorization url.String     `json:"authorizationUrl,omitempty"`
	Token         url.String     `json:"tokenUrl,omitempty"`
	Refresh       url.String     `json:"refreshUrl,omitempty"`
	Scopes        map[string]any `json:"scopes,omitempty"`
}

type oasPropertyName string

// Schema based on https://json-schema.org/draft/2020-12/json-schema-core
type oasSchema struct {
	ID uri.String `json:"$id,omitempty"`

	Defs map[string]*oasSchema `json:"$defs,omitempty"`

	Dialect uri.String `json:"$schema,omitempty"`
	Anchor  string     `json:"$anchor,omitempty"`

	AllOf []*oasSchema `json:"allOf,omitempty"`
	AnyOf []*oasSchema `json:"anyOf,omitempty"`
	OneOf []*oasSchema `json:"oneOf,omitempty"`
	Not   *oasSchema   `json:"not,omitempty"`

	Type []oasType `json:"type,omitempty"`

	Title        human.Readable                        `json:"title,omitempty"`
	Description  human.Readable                        `json:"description,omitempty"`
	Properties   map[oasPropertyName]*oasSchema        `json:"properties,omitempty"`
	Required     []oasPropertyName                     `json:"required,omitempty"`
	RequiredWhen map[oasPropertyName][]oasPropertyName `json:"dependentRequired,omitempty"`

	When map[oasPropertyName]*oasSchema `json:"if,omitempty"`

	If   *oasSchema `json:"if,omitempty"`
	Then *oasSchema `json:"then,omitempty"`
	Else *oasSchema `json:"else,omitempty"`

	MinLength  int     `json:"minLength,omitempty"`
	MaxLength  int     `json:"maxLength,omitempty"`
	MultipleOf float64 `json:"multipleOf,omitempty"`

	Default json.RawMessage `json:"default,omitempty"`

	Min *float64 `json:"minimum,omitempty"`
	Max *float64 `json:"maximum,omitempty"`

	MoreThan float64 `json:"exclusiveMinimum,omitempty"`
	LessThan float64 `json:"exclusiveMaximum,omitempty"`

	Const json.RawMessage   `json:"const,omitempty"`
	Enum  []json.RawMessage `json:"enum,omitempty"`
	Tuple []*oasSchema      `json:"prefixItems,omitempty"`

	Contains    *oasSchema `json:"contains,omitempty"`
	MinContains int        `json:"minContains,omitempty"`
	MaxContains int        `json:"maxContains,omitempty"`

	MinItems    int  `json:"minItems,omitempty"`
	MaxItems    int  `json:"maxItems,omitempty"`
	UniqueItems bool `json:"uniqueItems,omitempty"`

	Pattern           string                `json:"pattern,omitempty"`
	PatternProperties map[string]*oasSchema `json:"patternProperties,omitempty"`
	PropertyNames     *oasSchema            `json:"propertyNames,omitempty"`

	MinProperties int `json:"minProperties,omitempty"`
	MaxProperties int `json:"maxProperties,omitempty"`

	Items *oasSchema `json:"items,omitempty"`

	Format *oasFormat `json:"format,omitempty"`

	AdditionalProperties *oasSchema `json:"additionalProperties,omitempty"`

	ReadOnly  bool `json:"readOnly,omitempty"`
	WriteOnly bool `json:"writeOnly,omitempty"`

	Deprecated bool `json:"deprecated,omitempty"`

	ContentMediaType media.Type `json:"contentMediaType,omitempty"`
	ContentEncoding  string     `json:"contentEncoding,omitempty"`

	// OpenAPI extensions below

	Discriminator *oasDiscriminator         `json:"discriminator,omitempty"`
	XML           *oasXML                   `json:"xml,omitempty"`
	SeeAlso       *oasExternalDocumentation `json:"externalDocs,omitempty"`
	Example       json.RawMessage           `json:"example,omitempty"`
	Examples      []json.RawMessage         `json:"examples,omitempty"`
}

type oasProperty struct {
	Type        oasType        `json:"type,omitempty"`
	Description human.Readable `json:"description,omitempty"`
}

type oasType xyz.Switch[string, struct {
	String  oasType `txt:"string"`
	Number  oasType `txt:"number"`
	Integer oasType `txt:"integer"`
	Object  oasType `txt:"object"`
	Array   oasType `txt:"array"`
	Bool    oasType `txt:"boolean"`
	Null    oasType `txt:"null"`
}]

var oasTypes = xyz.AccessorFor(oasType.Values)

type oasFormat xyz.Switch[string, struct {
	DateTime oasFormat `txt:"date-time"`
	Time     oasFormat `txt:"time"`
	Date     oasFormat `txt:"date"`
	Duration oasFormat `txt:"duration"`
	Email    oasFormat `txt:"email"`
	Hostname oasFormat `txt:"hostname"`
	IPv4     oasFormat `txt:"ipv4"`
	IPv6     oasFormat `txt:"ipv6"`
	UUID     oasFormat `txt:"uuid"`
	URI      oasFormat `txt:"uri"`
	Regex    oasFormat `txt:"regex"`
}]
