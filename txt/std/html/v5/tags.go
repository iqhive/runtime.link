package html

import (
	"time"

	"runtime.link/ref/std/media"
	"runtime.link/ref/std/url"
	"runtime.link/txt/std/css/v3"
	"runtime.link/txt/std/human"
	"runtime.link/txt/std/src/js"
)

// Document is the root <html> element.
type Document struct {
	node `html:"<!DOCTYPE html><html"`
	ID   ID[Document] `html:" id=%q"`
	With Attributes   `html:"%v"`

	Namespace string `html:" xmlns=%q"`

	Tree node `html:">%v</html>"`
}

// Anchor is the <a> HTML element.
type Anchor struct {
	node `html:"<a"`
	ID   ID[Anchor] `html:" id=%q"`
	With Attributes `html:"%v"`

	Link           url.String     `html:" href=%q"`
	LinkLanguage   string         `html:" hreflang=%q"`
	Ping           []url.String   `html:" ping=%q"`
	Download       string         `html:" download=%q"`
	ReferrerPolicy ReferrerPolicy `html:" referrerpolicy=%q"`
	Relationship   Relationship   `html:" rel=%q"`
	Target         Target         `html:" target=%q"`
	Type           media.Type     `html:" type=%q"`

	Tree Nodes `html:">%v</a>"`
}

// Abbreviation is the <abbr> HTML element.
// Use the [Attributes.Tooltip] to specify
// the full text for the abbreviation.
type Abbreviation struct {
	node `html:"<abbr"`
	ID   ID[Abbreviation] `html:" id=%q"`
	With Attributes       `html:"%v"`
	Tree Nodes            `html:">%v</abbr>"`
}

// Address is the <address> HTML element.
type Address struct {
	node `html:"<address"`
	ID   ID[Address] `html:" id=%q"`
	With Attributes  `html:"%v"`
	Tree Nodes       `html:">%v</address>"`
}

// Article is the <article> HTML element.
type Article struct {
	node `html:"<article"`
	ID   ID[Article] `html:" id=%q"`
	With Attributes  `html:"%v"`
	Tree Nodes       `html:">%v</article>"`
}

// Aside is the <aside> HTML element.
type Aside struct {
	node `html:"<aside"`
	ID   ID[Aside]  `html:" id=%q"`
	With Attributes `html:"%v"`
	Tree Nodes      `html:">%v</aside>"`
}

// Audio is the <audio> HTML element.
type Audio struct {
	node `html:"<audio"`
	ID   ID[Audio]  `html:" id=%q"`
	With Attributes `html:"%v"`

	Source      url.String  `html:" src=%q"`
	Autoplay    bool        `html:" autoplay"`
	Controls    bool        `html:" controls"`
	CrossOrigin CrossOrigin `html:" crossorigin=%q"`
	Loop        bool        `html:" loop"`
	Muted       bool        `html:" muted"`
	Preload     Preload     `html:" preload=%q"`

	Tree Nodes `html:">%v</audio>"`
}

// BringAttentionTo is the <b> HTML element.
type BringAttentionTo struct {
	node `html:"<b"`
	ID   ID[BringAttentionTo] `html:" id=%q"`
	With Attributes           `html:"%v"`
	Tree Nodes                `html:">%v</b>"`
}

// Base is the <base> HTML element.
type Base struct {
	node `html:"<base"`
	ID   ID[Base]   `html:" id=%q"`
	With Attributes `html:"%v"`
	Tree Nodes      `html:">%v</base>"`
}

// BidirectionalIsolate is the <bdi> HTML element.
type BidirectionalIsolate struct {
	node `html:"<bdi"`
	ID   ID[BidirectionalIsolate] `html:" id=%q"`
	With Attributes               `html:"%v"`
	Tree Nodes                    `html:">%v</bdi>"`
}

// BidirectionalTextOverride is the <bdo> HTML element.
type BidirectionalTextOverride struct {
	node `html:"<bdo"`
	ID   ID[BidirectionalTextOverride] `html:" id=%q"`
	With Attributes                    `html:"%v"`
	Tree Nodes                         `html:">%v</bdo>"`
}

// Blockquote is the <blockquote> HTML element.
type Blockquote struct {
	node `html:"<blockquote"`
	ID   ID[Blockquote] `html:" id=%q"`
	With Attributes     `html:"%v"`
	Cite url.String     `html:" cite=%q"`
	Tree Nodes          `html:">%v</blockquote>"`
}

// Body is the <body> HTML element.
type Body struct {
	node `html:"<body"`
	ID   ID[Body]   `html:" id=%q"`
	With Attributes `html:"%v"`

	OnAfterPrint     js.Source `html:" onafterprint=%q"`
	OnBeforePrint    js.Source `html:" onbeforeprint=%q"`
	OnBeforeUnload   js.Source `html:" onbeforeunload=%q"`
	OnBlur           js.Source `html:" onblur=%q"`
	OnError          js.Source `html:" onerror=%q"`
	OnFocus          js.Source `html:" onfocus=%q"`
	OnHashChange     js.Source `html:" onhashchange=%q"`
	OnLanguageChange js.Source `html:" onlanguagechange=%q"`
	OnLoad           js.Source `html:" onload=%q"`
	OnMessage        js.Source `html:" onmessage=%q"`
	OnOffline        js.Source `html:" onoffline=%q"`
	OnOnline         js.Source `html:" ononline=%q"`
	OnPopState       js.Source `html:" onpopstate=%q"`
	OnRedo           js.Source `html:" onredo=%q"`
	OnResize         js.Source `html:" onresize=%q"`
	OnStorage        js.Source `html:" onstorage=%q"`
	OnUndo           js.Source `html:" onundo=%q"`
	OnUnload         js.Source `html:" onunload=%q"`

	Tree Nodes `html:">%v</body>"`
}

// Button is the <button> HTML element.
type Button struct {
	node                `html:"<button"`
	ID                  ID[Button]          `html:" id=%q"`
	With                Attributes          `html:"%v"`
	AutoFocus           bool                `html:" autofocus"`
	Disabled            bool                `html:" disabled"`
	Form                ID[Form]            `html:" form=%q"`
	FormAction          url.String          `html:" formaction=%q"`
	FormEncodingType    media.Type          `html:" formenctype=%q"`
	FormMethod          FormMethod          `html:" formmethod=%q"`
	FormNoValidate      bool                `html:" formnovalidate"`
	FormTarget          Target              `html:" formtarget=%q"`
	Name                string              `html:" name=%q"`
	PopoverTarget       ID[Node]            `html:" popovertarget=%q"`
	PopoverTargetAction PopoverTargetAction `html:" popovertargetaction=%q"`
	Type                ButtonType          `html:" type=%q"`
	Value               string              `html:" value=%q"`
	Tree                Node                `html:">%v</button>"`
}

// Canvas is the <canvas> HTML element.
type Canvas struct {
	node   `html:"<canvas"`
	ID     ID[Canvas] `html:" id=%q"`
	With   Attributes `html:"%v"`
	Height css.Pixels `html:" height=%q"`
	Width  css.Pixels `html:" width=%q"`
	Tree   Node       `html:">%v</canvas>"`
}

// Caption is the <caption> HTML element.
type Caption struct {
	node `html:"<caption"`
	ID   ID[Caption] `html:" id=%q"`
	With Attributes  `html:"%v"`
	Tree Nodes       `html:">%v</caption>"`
}

// Cite is the <cite> HTML element.
type Cite struct {
	node `html:"<cite"`
	ID   ID[Cite]   `html:" id=%q"`
	With Attributes `html:"%v"`
	Tree Nodes      `html:">%v</cite>"`
}

// SourceCode is the <code> HTML element.
type SourceCode struct {
	node `html:"<code"`
	ID   ID[SourceCode] `html:" id=%q"`
	With Attributes     `html:"%v"`
	Tree Nodes          `html:">%v</code>"`
}

// Column is the <col> HTML element.
type Column struct {
	node `html:"<col"`
	ID   ID[Column] `html:" id=%q"`
	With Attributes `html:"%v"`
	Span uint       `html:" span=\"%d\""`
	Tree Nodes      `html:">"`
}

// ColumnGroup is the <colgroup> HTML element.
type ColumnGroup struct {
	node `html:"<colgroup"`
	ID   ID[ColumnGroup] `html:" id=%q"`
	With Attributes      `html:"%v"`
	Span uint            `html:" span=\"%d\""`
	Tree Nodes           `html:">%v</colgroup>"`
}

// MachineReadable is the <data> HTML element.
type MachineReadable struct {
	node  `html:"<data"`
	ID    ID[MachineReadable] `html:" id=%q"`
	With  Attributes          `html:"%v"`
	Value string              `html:" value=%q"`
	Tree  Node                `html:">%v</data>"`
}

// OptionList is the <datalist> HTML element.
type OptionList struct {
	node `html:"<datalist"`
	ID   ID[OptionList] `html:" id=%q"`
	With Attributes     `html:"%v"`
	Tree Nodes          `html:">%v</datalist>"`
}

// DescriptionDetails is the <dd> HTML element.
type DescriptionDetails struct {
	node `html:"<dd"`
	ID   ID[DescriptionDetails] `html:" id=%q"`
	With Attributes             `html:"%v"`
	Tree Nodes                  `html:">%v</dd>"`
}

// DeletedText is the <del> HTML element.
type DeletedText struct {
	node `html:"<del"`
	ID   ID[DeletedText] `html:" id=%q"`
	With Attributes      `html:"%v"`
	Tree Nodes           `html:">%v</del>"`
}

// Details is the <details> HTML element.
type Details struct {
	node `html:"<details"`
	ID   ID[Details] `html:" id=%q"`
	With Attributes  `html:"%v"`
	Open bool        `html:" open"`
	Tree Nodes       `html:">%v</details>"`
}

// Definition is the <dfn> HTML element.
// Use the [Attributes.Tooltip] to specify
// the full text for the definition.
type Definition struct {
	node `html:"<dfn"`
	ID   ID[Definition] `html:" id=%q"`
	With Attributes     `html:"%v"`
	Tree Nodes          `html:">%v</dfn>"`
}

// Dialog is the <dialog> HTML element.
type Dialog struct {
	node `html:"<dialog"`
	ID   ID[Dialog] `html:" id=%q"`
	With Attributes `html:"%v"`
	Open bool       `html:" open"`
	Tree Nodes      `html:">%v</dialog>"`
}

// Container is the <div> HTML element.
type Container struct {
	node `html:"<div"`
	ID   ID[Container] `html:" id=%q"`
	With Attributes    `html:"%v"`
	Tree Nodes         `html:">%v</div>"`
}

// DescriptionList is the <dl> HTML element.
type DescriptionList struct {
	node `html:"<dl"`
	ID   ID[DescriptionList] `html:" id=%q"`
	With Attributes          `html:"%v"`
	Tree Nodes               `html:">%v</dl>"`
}

// DescriptionTerm is the <dt> HTML element.
type DescriptionTerm struct {
	node `html:"<dt"`
	ID   ID[DescriptionTerm] `html:" id=%q"`
	With Attributes          `html:"%v"`
	Tree Nodes               `html:">%v</dt>"`
}

// Emphasis is the <em> HTML element.
type Emphasis struct {
	node `html:"<em"`
	ID   ID[Emphasis] `html:" id=%q"`
	With Attributes   `html:"%v"`
	Tree Nodes        `html:">%v</em>"`
}

// Embed is the <embed> HTML element.
type Embed struct {
	node   `html:"<embed"`
	ID     ID[Embed]  `html:" id=%q"`
	With   Attributes `html:"%v"`
	Source url.String `html:" src=%q"`
	Height css.Pixels `html:" height=%q"`
	Width  css.Pixels `html:" width=%q"`
	Type   media.Type `html:" type=%q"`
	Tree   Node       `html:">"`
}

// FieldSet is the <fieldset> HTML element.
type FieldSet struct {
	node     `html:"<fieldset"`
	ID       ID[FieldSet] `html:" id=%q"`
	With     Attributes   `html:"%v"`
	Disabled bool         `html:" disabled"`
	Form     ID[Form]     `html:" form=%q"`
	Name     string       `html:" name=%q"`
	Tree     Node         `html:">%v</fieldset>"`
}

// FigureCaption is the <figcaption> HTML element.
type FigureCaption struct {
	node `html:"<figcaption"`
	ID   ID[FigureCaption] `html:" id=%q"`
	With Attributes        `html:"%v"`
	Tree Nodes             `html:">%v</figcaption>"`
}

// Figure is the <figure> HTML element.
type Figure struct {
	node `html:"<figure"`
	ID   ID[Figure] `html:" id=%q"`
	With Attributes `html:"%v"`
	Tree Nodes      `html:">%v</figure>"`
}

// Footer is the <footer> HTML element.
type Footer struct {
	node `html:"<footer"`
	ID   ID[Footer] `html:" id=%q"`
	With Attributes `html:"%v"`
	Tree Nodes      `html:">%v</footer>"`
}

// Form is the <form> HTML element.
type Form struct {
	node               `html:"<form"`
	ID                 ID[Form]     `html:" id=%q"`
	With               Attributes   `html:"%v"`
	AcceptCharacterSet CharacterSet `html:" accept-charset=%q"`
	AutoComplete       OnOff        `html:" autocomplete=%q"`
	Name               string       `html:" name=%q"`
	Relationship       Relationship `html:" rel=%q"`
	Action             url.String   `html:" action=%q"`
	EncodingType       media.Type   `html:" enctype=%q"`
	Method             FormMethod   `html:" method=%q"`
	NoValidate         bool         `html:" novalidate"`
	Target             Target       `html:" target=%q"`
	Tree               Node         `html:">%v</form>"`
}

// Heading1 is the <h1> HTML element.
type Heading1 struct {
	node `html:"<h1"`
	ID   ID[Heading1] `html:" id=%q"`
	With Attributes   `html:"%v"`
	Tree Nodes        `html:">%v</h1>"`
}

// Heading2 is the <h2> HTML element.
type Heading2 struct {
	node `html:"<h2"`
	ID   ID[Heading2] `html:" id=%q"`
	With Attributes   `html:"%v"`
	Tree Nodes        `html:">%v</h2>"`
}

// Heading3 is the <h3> HTML element.
type Heading3 struct {
	node `html:"<h3"`
	ID   ID[Heading2] `html:" id=%q"`
	With Attributes   `html:"%v"`
	Tree Nodes        `html:">%v</h3>"`
}

// Heading4 is the <h4> HTML element.
type Heading4 struct {
	node `html:"<h4"`
	ID   ID[Heading2] `html:" id=%q"`
	With Attributes   `html:"%v"`
	Tree Nodes        `html:">%v</h4>"`
}

// Heading5 is the <h5> HTML element.
type Heading5 struct {
	node `html:"<h5"`
	ID   ID[Heading2] `html:" id=%q"`
	With Attributes   `html:"%v"`
	Tree Nodes        `html:">%v</h5>"`
}

// Heading6 is the <h6> HTML element.
type Heading6 struct {
	node `html:"<h6"`
	ID   ID[Heading2] `html:" id=%q"`
	With Attributes   `html:"%v"`
	Tree Nodes        `html:">%v</h6>"`
}

// Head is the <head> HTML element.
type Head struct {
	node `html:"<head"`
	ID   ID[Head]   `html:" id=%q"`
	With Attributes `html:"%v"`
	Tree Nodes      `html:">%v</head>"`
}

// Header is the <header> HTML element.
type Header struct {
	node `html:"<header"`
	ID   ID[Header] `html:" id=%q"`
	With Attributes `html:"%v"`
	Tree Nodes      `html:">%v</header>"`
}

// HeadingGroup is the <hgroup> HTML element.
type HeadingGroup struct {
	node `html:"<hgroup"`
	ID   ID[HeadingGroup] `html:" id=%q"`
	With Attributes       `html:"%v"`
	Tree Nodes            `html:">%v</hgroup>"`
}

// ThematicBreak is the <hr> HTML element.
type ThematicBreak struct {
	node `html:"<hr"`
	ID   ID[ThematicBreak] `html:" id=%q"`
	With Attributes        `html:"%v"`
	Tree Nodes             `html:">"`
}

// IdiomaticText is the <i> HTML element.
type IdiomaticText struct {
	node `html:"<i"`
	ID   ID[IdiomaticText] `html:" id=%q"`
	With Attributes        `html:"%v"`
	Tree Nodes             `html:">%v</i>"`
}

// InlineFrame is the <iframe> HTML element.
type InlineFrame struct {
	node           `html:"<iframe"`
	ID             ID[InlineFrame]   `html:" id=%q"`
	Source         url.String        `html:" src=%q"`
	SourceDocument String            `html:" srcdoc=%q"`
	With           Attributes        `html:"%v"`
	Allow          PermissionsPolicy `html:" allow=%q"`
	Height         css.Pixels        `html:" height=%q"`
	Width          css.Pixels        `html:" width=%q"`
	Loading        Loading           `html:" loading=%q"`
	Name           Target            `html:" name=%q"`
	ReferrerPolicy ReferrerPolicy    `html:" referrerpolicy=%q"`
	Sandbox        []Sandbox         `html:" sandbox=%q"`
	Tree           Node              `html:">%v</iframe>"`
}

// Image is the <img> HTML element.
type Image struct {
	node           `html:"<img"`
	ID             ID[Image]             `html:" id=%q"`
	Source         url.String            `html:" src=%q"`
	SourceSet      []string              `html:" srcset=%q"`
	With           Attributes            `html:"%v"`
	Alternative    human.Readable        `html:" alt=%q"`
	CrossOrigin    CrossOrigin           `html:" crossorigin=%q"`
	Decoding       ImageDecodingStrategy `html:" decoding=%q"`
	ElementTiming  bool                  `html:" elementtiming"`
	Width          css.Pixels            `html:" width=%q"`
	Height         css.Pixels            `html:" height=%q"`
	IsMap          bool                  `html:" ismap"`
	UseMap         url.String            `html:" usemap=%q"`
	Loading        Loading               `html:" loading=%q"`
	ReferrerPolicy ReferrerPolicy        `html:" referrerpolicy=%q"`
	Sizes          []string              `html:" sizes=%q"`
	Tree           Node                  `html:">"`
}

// Input is the <input> HTML element.
type Input struct {
	node                `html:"<input"`
	ID                  ID[Input]           `html:" id=%q"`
	Type                InputType           `html:" type=%q"`
	Accept              media.Type          `html:" accept=%q"`
	Alternative         human.Readable      `html:" alt=%q"`
	AutoComplete        OnOff               `html:" autocomplete=%q"`
	Capture             string              `html:" capture=%q"`
	Checked             bool                `html:" checked"`
	DirectionName       string              `html:" dirname=%q"`
	Disabled            bool                `html:" disabled"`
	Form                ID[Form]            `html:" form=%q"`
	FormAction          url.String          `html:" formaction=%q"`
	FormEncodingType    media.Type          `html:" formenctype=%q"`
	FormMethod          FormMethod          `html:" formmethod=%q"`
	FormNoValidate      bool                `html:" formnovalidate"`
	FormTarget          Target              `html:" formtarget=%q"`
	Height              css.Pixels          `html:" height=%q"`
	List                ID[OptionList]      `html:" list=%q"`
	Max                 float64             `html:" max=%q"`
	MaxLength           uint                `html:" maxlength=\"%d\""`
	Min                 float64             `html:" min=%q"`
	MinLength           uint                `html:" minlength=\"%d\""`
	Multiple            bool                `html:" multiple"`
	Name                string              `html:" name=%q"`
	Pattern             string              `html:" pattern=%q"`
	Placeholder         string              `html:" placeholder=%q"`
	PopoverTarget       bool                `html:" popovertarget"`
	PopoverTargetAction PopoverTargetAction `html:" popovertargetaction=%q"`
	ReadOnly            bool                `html:" readonly"`
	Required            bool                `html:" required"`
	Size                css.Pixels          `html:" size=\"%d\""`
	Source              url.String          `html:" src=%q"`
	Value               string              `html:" value=%q"`
	Width               css.Pixels          `html:" width=%q"`
	With                Attributes          `html:"%v"`
	Tree                Node                `html:">"`
}

// ImageMapName for an [ImageMap].
type ImageMapName string

// ImageMap is the <map> HTML element.
type ImageMap struct {
	node  `html:"<map"`
	ID    ID[ImageMap]   `html:" id=%q"`
	With  Attributes     `html:"%v"`
	Name  ImageMapName   `html:" name=%q"`
	Areas []ImageMapArea `html:">%v</map>"`
	Tree  Node           `html:">%v</map>"`
}

// InsertedText is the <ins> HTML element.
type InsertedText struct {
	node     `html:"<ins"`
	ID       ID[InsertedText] `html:" id=%q"`
	With     Attributes       `html:"%v"`
	Cite     url.String       `html:" cite=%q"`
	DateTime time.Time        `html:" datetime=%q"`
	Tree     Node             `html:">%v</ins>"`
}

// ImageMapArea is the <area> HTML element.
type ImageMapArea struct {
	node `html:"<area"`
	ID   ID[ImageMapArea] `html:" id=%q"`
	With Attributes       `html:"%v"`

	Alternative human.Readable `html:" alt=%q"`
	Coords      []float64      `html:" coords=%q"`

	Link           url.String     `html:" href=%q"`
	Download       string         `html:" download=%q"`
	Ping           []url.String   `html:" ping=%q"`
	ReferrerPolicy ReferrerPolicy `html:" referrerpolicy=%q"`
	Relationship   Relationship   `html:" rel=%q"`
	Shape          ImageMapShape  `html:" shape=%q"`
	Target         Target         `html:" target=%q"`

	Tree Nodes `html:">%v</area>"`
}

// KeyboardInput is the <kbd> HTML element.
type KeyboardInput struct {
	node `html:"<kbd"`
	ID   ID[KeyboardInput] `html:" id=%q"`
	With Attributes        `html:"%v"`
	Tree Nodes             `html:">%v</kbd>"`
}

// Label is the <label> HTML element.
type Label struct {
	node `html:"<label"`
	ID   ID[Label]  `html:" id=%q"`
	With Attributes `html:"%v"`
	For  ID[Node]   `html:" for=%q"`
	Tree Nodes      `html:">%v</label>"`
}

// Legend is the <legend> HTML element.
type Legend struct {
	node `html:"<legend"`
	ID   ID[Legend] `html:" id=%q"`
	With Attributes `html:"%v"`
	Tree Nodes      `html:">%v</legend>"`
}

// ListItem is the <li> HTML element.
type ListItem struct {
	node  `html:"<li"`
	ID    ID[ListItem] `html:" id=%q"`
	With  Attributes   `html:"%v"`
	Value uint         `html:" value=\"%d\""`
	Tree  Node         `html:">%v</li>"`
}

// Link is the <link> HTML element.
type Link struct {
	node             `html:"<link"`
	ID               ID[Link]       `html:" id=%q"`
	With             Attributes     `html:"%v"`
	As               LinkType       `html:" as=%q"`
	CrossOrigin      CrossOrigin    `html:" crossorigin=%q"`
	Location         url.String     `html:" href=%q"`
	LocationLangauge string         `html:" hreflang=%q"`
	ImageSizes       []string       `html:" imagesizes=%q"`
	ImageSourceSet   []string       `html:" imagesrcset=%q"`
	Integrity        string         `html:" integrity=%q"`
	Media            media.Type     `html:" media=%q"`
	ReferrerPolicy   ReferrerPolicy `html:" referrerpolicy=%q"`
	Relationship     Relationship   `html:" rel=%q"`
	Type             media.Type     `html:" type=%q"`
	Tree             Node           `html:">"`
}

// Highlight is the <mark> HTML element.
type Highlight struct {
	node `html:"<mark"`
	ID   ID[Highlight] `html:" id=%q"`
	With Attributes    `html:"%v"`
	Tree Nodes         `html:">%v</mark>"`
}

// Main is the <main> HTML element.
type Main struct {
	node `html:"<main"`
	ID   ID[Main]   `html:" id=%q"`
	With Attributes `html:"%v"`
	Tree Nodes      `html:">%v</main>"`
}

// Menu is the <menu> HTML element.
type Menu struct {
	node `html:"<menu"`
	ID   ID[Menu]   `html:" id=%q"`
	With Attributes `html:"%v"`
	Tree Nodes      `html:">%v</menu>"`
}

// Metadata is the <meta> HTML element.
type Metadata struct {
	CharacterSet     CharacterSet `html:" charset=%q"`
	HeaderEquivalent string       `html:" http-equiv=%q"`
	Name             string       `html:" name=%q"`
	Content          string       `html:" content=%q"`
}

// Meter is the <meter> HTML element.
type Meter struct {
	node `html:"<meter"`
	ID   ID[Meter]  `html:" id=%q"`
	With Attributes `html:"%v"`
	Tree Nodes      `html:">%v</meter>"`
}

// Navigation is the <nav> HTML element.
type Navigation struct {
	node `html:"<nav"`
	ID   ID[Navigation] `html:" id=%q"`
	With Attributes     `html:"%v"`
	Tree Nodes          `html:">%v</nav>"`
}

// WhenScriptIsDisabled is the <noscript> HTML element.
type WhenScriptIsDisabled struct {
	node `html:"<noscript"`
	ID   ID[WhenScriptIsDisabled] `html:" id=%q"`
	With Attributes               `html:"%v"`
	Tree Nodes                    `html:">%v</noscript>"`
}

// Object is the <object> HTML element.
type Object struct {
	node   `html:"<object"`
	ID     ID[Object] `html:" id=%q"`
	Name   string     `html:" name=%q"`
	Data   url.String `html:" data=%q"`
	With   Attributes `html:"%v"`
	Height css.Pixels `html:" height=%q"`
	Width  css.Pixels `html:" width=%q"`
	Type   media.Type `html:" type=%q"`
	UseMap url.String `html:" usemap=%q"`
	Tree   Node       `html:">%v</object>"`
}

// OrderedList is the <ol> HTML element.
type OrderedList struct {
	node     `html:"<ol"`
	ID       ID[OrderedList] `html:" id=%q"`
	Reversed bool            `html:" reversed"`
	Start    uint            `html:" start=\"%d\""`
	With     Attributes      `html:"%v"`
	Type     rune            `html:" type=%q"`
	Tree     Node            `html:">%v</ol>"`
}

// OptionGroup is the <optgroup> HTML element.
type OptionGroup struct {
	node     `html:"<optgroup"`
	ID       ID[OptionGroup] `html:" id=%q"`
	Disabled bool            `html:" disabled"`
	Label    string          `html:" label=%q"`
	With     Attributes      `html:"%v"`
	Tree     Node            `html:">%v</optgroup>"`
}

// Option is the <option> HTML element.
type Option struct {
	node     `html:"<option"`
	ID       ID[Option] `html:" id=%q"`
	Disabled bool       `html:" disabled"`
	Label    string     `html:" label=%q"`
	Selected bool       `html:" selected"`
	Value    string     `html:" value=%q"`
	With     Attributes `html:"%v"`
	Text     string     `html:">%v</option>"`
}

// Output is the <output> HTML element.
type Output struct {
	node `html:"<output"`
	ID   ID[Output] `html:" id=%q"`
	For  []ID[Node] `html:" for=%q"`
	Form ID[Form]   `html:" form=%q"`
	Name string     `html:" name=%q"`
	With Attributes `html:"%v"`
	Tree Nodes      `html:">%v</output>"`
}

// Paragraph is the <p> HTML element.
type Paragraph struct {
	node `html:"<p"`
	ID   ID[Paragraph] `html:" id=%q"`
	With Attributes
	Tree Nodes `html:">%v</p>"`
}

// Picture is the <picture> HTML element.
type Picture struct {
	node    `html:"<picture"`
	ID      ID[Picture] `html:" id=%q"`
	With    Attributes  `html:"%v"`
	Sources []Source    `html:">%v"`
	Images  []Image     `html:">%v</picture>"`
}

// Progress is the <progress> HTML element.
type Progress struct {
	node  `html:"<progress"`
	ID    ID[Progress] `html:" id=%q"`
	With  Attributes   `html:"%v"`
	Value float64      `html:" value=%q"`
	Max   float64      `html:" max=%q"`
	Tree  Node         `html:">%v</progress>"`
}

// PreformattedText is the <pre> HTML element.
type PreformattedText struct {
	node `html:"<pre"`
	ID   ID[PreformattedText] `html:" id=%q"`
	With Attributes           `html:"%v"`
	Tree Nodes                `html:">%v</pre>"`
}

// Quotation is the <q> HTML element.
type Quotation struct {
	node `html:"<q"`
	ID   ID[Quotation] `html:" id=%q"`
	With Attributes    `html:"%v"`
	Tree Nodes         `html:">%v</q>"`
}

// RubyFallback is the <rp> HTML element.
type RubyFallback struct {
	node `html:"<rp"`
	ID   ID[RubyFallback] `html:" id=%q"`
	With Attributes       `html:"%v"`
	Tree Nodes            `html:">%v</rp>"`
}

// RubyText is the <rt> HTML element.
type RubyText struct {
	node `html:"<rt"`
	ID   ID[RubyText] `html:" id=%q"`
	With Attributes   `html:"%v"`
	Tree Nodes        `html:">%v</rt>"`
}

// Ruby is the <ruby> HTML element.
type Ruby struct {
	node `html:"<ruby"`
	ID   ID[Ruby] `html:" id=%q"`
	With Attributes
	Tree Nodes `html:">%v</ruby>"`
}

// Strikethrough is the <s> HTML element.
type Strikethrough struct {
	node `html:"<s"`
	ID   ID[Strikethrough] `html:" id=%q"`
	With Attributes        `html:"%v"`
	Tree Nodes             `html:">%v</s>"`
}

// SampleOutput is the <samp> HTML element.
type SampleOutput struct {
	node `html:"<samp"`
	ID   ID[SampleOutput] `html:" id=%q"`
	With Attributes       `html:"%v"`
	Tree Nodes            `html:">%v</samp>"`
}

// Script is the <script> HTML element.
type Script struct {
	node           `html:"<script"`
	ID             ID[Script]     `html:" id=%q"`
	With           Attributes     `html:"%v"`
	Async          bool           `html:" async"`
	Defer          bool           `html:" defer"`
	Source         url.String     `html:" src=%q"`
	SourceText     String         `html:" srctext=%q"`
	Type           ScriptType     `html:" type=%q"`
	ReferrerPolicy ReferrerPolicy `html:" referrerpolicy=%q"`
	CrossOrigin    CrossOrigin    `html:" crossorigin=%q"`
	Integrity      string         `html:" integrity=%q"`
	NoModule       bool           `html:" nomodule"`
	Nonce          string         `html:" nonce=%q"`
	Tree           Node           `html:">%v</script>"`
}

// Search is the <script> HTML element.
type Search struct {
	node `html:"<search"`
	ID   ID[Search] `html:" id=%q"`
	With Attributes
	Tree Nodes `html:">%v</search>"`
}

// Section is the <section> HTML element.
type Section struct {
	node `html:"<section"`
	ID   ID[Section] `html:" id=%q"`
	With Attributes  `html:"%v"`
	Tree Nodes       `html:">%v</section>"`
}

// SelectFromOptions is the <select> HTML element.
type SelectFromOptions struct {
	node         `html:"<select"`
	ID           ID[SelectFromOptions] `html:" id=%q"`
	With         Attributes            `html:"%v"`
	AutoComplete OnOff                 `html:" autocomplete=%q"`
	AutoFocus    bool                  `html:" autofocus"`
	Disabled     bool                  `html:" disabled"`
	Form         ID[Form]              `html:" form=%q"`
	Multiple     bool                  `html:" multiple"`
	Name         string                `html:" name=%q"`
	Required     bool                  `html:" required"`
	Size         uint                  `html:" size=\"%d\""`
	Options      []Option              `html:">%v</select>"`
}

// SelectFromOptionGroups is the <select> HTML element.
type SelectFromOptionGroups struct {
	node         `html:"<select"`
	ID           ID[SelectFromOptions] `html:" id=%q"`
	With         Attributes            `html:"%v"`
	AutoComplete OnOff                 `html:" autocomplete=%q"`
	AutoFocus    bool                  `html:" autofocus"`
	Disabled     bool                  `html:" disabled"`
	Form         ID[Form]              `html:" form=%q"`
	Multiple     bool                  `html:" multiple"`
	Name         string                `html:" name=%q"`
	Required     bool                  `html:" required"`
	Size         uint                  `html:" size=\"%d\""`
	Options      []OptionGroup         `html:">%v</select>"`
}

// Slot is the <slot> HTML element.
type Slot struct {
	node `html:"<slot"`
	ID   ID[Slot] `html:" id=%q"`
	With Attributes
	Tree Nodes `html:">%v</slot>"`
}

// Fineprint is the <small> HTML element.
type Fineprint struct {
	node `html:"<small"`
	ID   ID[Fineprint] `html:" id=%q"`
	With Attributes
	Tree Nodes `html:">%v</small>"`
}

// Source is the <source> HTML element.
type Source struct {
	node      `html:"<source"`
	ID        ID[Source] `html:" id=%q"`
	Source    url.String `html:" src=%q"`
	With      Attributes `html:"%v"`
	SourceSet []string   `html:" srcset=%q"`
	Sizes     []string   `html:" sizes=%q"`
	Media     media.Type `html:" media=%q"`
	Height    css.Pixels `html:" height=%q"`
	Width     css.Pixels `html:" width=%q"`
	Type      media.Type `html:" type=%q"`
	void      struct{}   `html:">"`
}

// Span is the <span> HTML element.
type Span struct {
	node `html:"<span"`
	ID   ID[Span]   `html:" id=%q"`
	With Attributes `html:"%v"`
	Tree Nodes      `html:">%v</span>"`
}

// Strong is the <strong> HTML element.
type Strong struct {
	node `html:"<strong"`
	ID   ID[Strong] `html:" id=%q"`
	With Attributes `html:"%v"`
	Tree Nodes      `html:">%v</strong>"`
}

// Style is the <style> HTML element.
type Style struct {
	node  `html:"<style"`
	ID    ID[Style]  `html:" id=%q"`
	With  Attributes `html:"%v"`
	Media string     `html:" media=%q"`
	Nonce string     `html:" nonce=%q"`
	Text  css.String `html:">%v</style>"`
}

// Subscript is the <sub> HTML element.
type Subscript struct {
	node `html:"<sub"`
	ID   ID[Subscript] `html:" id=%q"`
	With Attributes    `html:"%v"`
	Tree Nodes         `html:">%v</sub>"`
}

// Summary is the <summary> HTML element.
type Summary struct {
	node `html:"<summary"`
	ID   ID[Summary] `html:" id=%q"`
	With Attributes  `html:"%v"`
	Tree Nodes       `html:">%v</summary>"`
}

// Superscript is the <sup> HTML element.
type Superscript struct {
	node `html:"<sup"`
	ID   ID[Superscript] `html:" id=%q"`
	With Attributes      `html:"%v"`
	Tree Nodes           `html:">%v</sup>"`
}

// Table is the <table> HTML element.
type Table struct {
	node `html:"<table"`
	ID   ID[Table]  `html:" id=%q"`
	With Attributes `html:"%v"`
	Tree Nodes      `html:">%v</table>"`
}

// TableBody is the <tbody> HTML element.
type TableBody struct {
	node `html:"<tbody"`
	ID   ID[TableBody] `html:" id=%q"`
	With Attributes    `html:"%v"`
	Tree Nodes         `html:">%v</tbody>"`
}

// TableData is the <td> HTML element.
type TableData struct {
	node       `html:"<td"`
	ID         ID[TableData]      `html:" id=%q"`
	With       Attributes         `html:"%v"`
	RowSpan    uint               `html:" rowspan=\"%d\""`
	ColumnSpan uint               `html:" colspan=\"%d\""`
	Headings   []ID[TableHeading] `html:" headers=%q"`
	Tree       Node               `html:">%v</td>"`
}

// Template is the <template> HTML element.
type Template struct {
	node `html:"<template"`
	ID   ID[Template] `html:" id=%q"`
	With Attributes   `html:"%v"`
	Tree Nodes        `html:">%v</template>"`
}

// TextArea is the <textarea> HTML element.
type TextArea struct {
	node          `html:"<textarea"`
	ID            ID[TextArea] `html:" id=%q"`
	With          Attributes   `html:"%v"`
	AutoComplete  OnOff        `html:" autocomplete=%q"`
	AutoFocus     bool         `html:" autofocus"`
	Columns       uint         `html:" cols=\"%d\""`
	DirectionName string       `html:" dirname=%q"`
	Disabled      bool         `html:" disabled"`
	Form          ID[Form]     `html:" form=%q"`
	MaxLength     uint         `html:" maxlength=\"%d\""`
	MinLength     uint         `html:" minlength=\"%d\""`
	Name          string       `html:" name=%q"`
	Placeholder   string       `html:" placeholder=%q"`
	ReadOnly      bool         `html:" readonly"`
	Required      bool         `html:" required"`
	Rows          uint         `html:" rows=\"%d\""`
	Spellcheck    Bool         `html:" spellcheck=%q"`
	Wrap          Wrap         `html:" wrap=%q"`
	Tree          Node         `html:">%v</textarea>"`
}

// TableFooter is the <tfoot> HTML element.
type TableFooter struct {
	node `html:"<tfoot"`
	ID   ID[TableFooter] `html:" id=%q"`
	With Attributes      `html:"%v"`
	Tree Nodes           `html:">%v</tfoot>"`
}

// TableHeading is the <th> HTML element.
type TableHeading struct {
	node         `html:"<th"`
	ID           ID[TableHeading]   `html:" id=%q"`
	With         Attributes         `html:"%v"`
	Abbreviation string             `html:" abbr=%q"`
	RowSpan      uint               `html:" rowspan=\"%d\""`
	ColumnSpan   uint               `html:" colspan=\"%d\""`
	Headers      []ID[TableHeading] `html:" headers=%q"`
	Scope        TableScope         `html:" scope=%q"`
	Tree         Node               `html:">%v</th>"`
}

// TableHeader is the <thead> HTML element.
type TableHeader struct {
	node `html:"<thead"`
	ID   ID[TableHeader] `html:" id=%q"`
	With Attributes      `html:"%v"`
	Tree Nodes           `html:">%v</thead>"`
}

// Time is the <time> HTML element.
type Time struct {
	node     `html:"<time"`
	ID       ID[Time]   `html:" id=%q"`
	With     Attributes `html:"%v"`
	DateTime time.Time  `html:" datetime=%q"`
	Tree     Node       `html:">%v</time>"`
}

// Title is the <title> HTML element.
type Title struct {
	node `html:"<title"`
	ID   ID[Title]  `html:" id=%q"`
	With Attributes `html:"%v"`
	Text string     `html:">%v</title>"`
}

// TableRow is the <tr> HTML element.
type TableRow struct {
	node `html:"<tr"`
	ID   ID[TableRow] `html:" id=%q"`
	With Attributes   `html:"%v"`
	Tree Nodes        `html:">%v</tr>"`
}

// Track is the <track> HTML element.
type Track struct {
	node           `html:"<track"`
	ID             ID[Track]  `html:" id=%q"`
	With           Attributes `html:"%v"`
	Source         url.String `html:" src=%q"`
	SourceLanguage string     `html:" srclang=%q"`
	Default        bool       `html:" default"`
	Kind           TrackKind  `html:" kind=%q"`
	Label          string     `html:" label=%q"`
	void           struct{}   `html:">"`
}

// Annotation is the <u> HTML element.
type Annotation struct {
	node `html:"<u"`
	ID   ID[Annotation] `html:" id=%q"`
	With Attributes     `html:"%v"`
	Tree Nodes          `html:">%v</u>"`
}

// UnorderedList is the <ul> HTML element.
type UnorderedList struct {
	node `html:"<ul"`
	ID   ID[UnorderedList] `html:" id=%q"`
	With Attributes        `html:"%v"`
	Tree Nodes             `html:">%v</ul>"`
}

// MathematicalVariable is the <var> HTML element.
type MathematicalVariable struct {
	node `html:"<var"`
	ID   ID[MathematicalVariable] `html:" id=%q"`
	With Attributes               `html:"%v"`
	Tree Nodes                    `html:">%v</var>"`
}

// Video is the <video> HTML element.
type Video struct {
	node        `html:"<video"`
	ID          ID[Video]      `html:" id=%q"`
	Source      url.String     `html:" src=%q"`
	SourceSet   []string       `html:" srcset=%q"`
	With        Attributes     `html:"%v"`
	Alternative human.Readable `html:" alt=%q"`
	AutoPlay    bool           `html:" autoplay"`
	Controls    bool           `html:" controls"`
	CrossOrigin CrossOrigin    `html:" crossorigin=%q"`
	Height      css.Pixels     `html:" height=%q"`
	Loop        bool           `html:" loop"`
	Muted       bool           `html:" muted"`
	PlaysInline bool           `html:" playsinline"`
	Poster      url.String     `html:" poster=%q"`
	Preload     Preload        `html:" preload=%q"`
	Width       css.Pixels     `html:" width=%q"`
	void        struct{}       `html:">"`
}

// LineBreak is the <br> HTML element.
type LineBreak struct {
	node `html:"<br"`
	ID   ID[LineBreak] `html:" id=%q"`
	With Attributes    `html:"%v"`
	Tree Nodes         `html:">"`
}

// OptionalLineBreak is the <wbr> HTML element.
type OptionalLineBreak struct {
	node `html:"<wbr"`
	ID   ID[OptionalLineBreak] `html:" id=%q"`
	With Attributes            `html:"%v"`
	void struct{}              `html:">"`
}
