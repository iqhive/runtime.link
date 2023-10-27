package html

import (
	"fmt"
	"html"
	"reflect"
	"strings"

	"runtime.link/ref/std/url"
	"runtime.link/txt/std/css/v3"
)

// Element is an HTML5 element.
type Node interface{ HTML5() }

// Value returns a fmt.Formatter for the given element.
func Value(node Node) fmt.Formatter {
	if already, ok := node.(fmt.Formatter); ok {
		return already
	}
	if nodes, ok := node.(Nodes); ok {
		return formatter{nodes}
	}
	return formatter{nodes: Nodes{node}}
}

type formatter struct {
	nodes Nodes
}

func (f formatter) Format(w fmt.State, verb rune) {
	for _, element := range f.nodes {
		if already, ok := element.(fmt.Formatter); ok {
			already.Format(w, verb)
			return
		}
		rvalue := reflect.ValueOf(element)
		for rvalue.Kind() == reflect.Ptr {
			rvalue = rvalue.Elem()
		}
		rtype := rvalue.Type()
		for i := 0; i < rtype.NumField(); i++ {
			tag := rtype.Field(i).Tag.Get("html")
			if tag == "" {
				continue
			}
			if !rtype.Field(i).IsExported() {
				if !strings.Contains(tag, "%") {
					fmt.Fprintf(w, tag)
				}
				continue
			}
			if rtype.Field(i).Type.Kind() == reflect.Bool && !rvalue.Field(i).Bool() {
				continue
			}
			field := rvalue.Field(i)
			value := field.Interface()
			kind := field.Kind()
			if element, ok := value.(Node); ok {
				fmt.Fprintf(w, tag, Value(element))
			} else {
				if field.IsZero() {
					if (kind != reflect.Struct && kind != reflect.Interface) || tag == "%v" {
						continue
					}
					value = ""
				}
				fmt.Fprintf(w, tag, value)
			}
		}
	}
}

type Nodes []Node

func (e Nodes) HTML5() {}

type node Node

// String containing HTMLv5.
type String string

// Class name.
type Class string

// ID name.
type ID[T node] string

// CustomElement name.
type CustomElement string

// ItemID name.
type ItemID string

// ItemProperty name.
type ItemProperty string

// Part name.
type Part string

// SlotName string.
type SlotName string

// Attributes common to all elements.
type Attributes struct {
	node

	AccessKey        []string           `html:" accesskey=%q"`
	AutoCapitalize   AutoCapitalization `html:" autocapitalize=%q"`
	AutoFocus        bool               `html:" autofocus"`
	Class            []Class            `html:" class=%q"`
	ContentEditable  Editablility       `html:" contenteditable=%q"`
	ContextMenu      ID[Menu]           `html:" contextmenu=%q"`
	Directionality   Directionality     `html:" dir=%q"`
	Draggable        Bool               `html:" draggable=%q"`
	EnterKeyHint     string             `html:" enterkeyhint=%q"`
	Hidden           bool               `html:" hidden"`
	HiddenUntilFound bool               `html:" hidden=\"untilfound\""`

	Inert     bool          `html:" inert"`
	InputMode InputMode     `html:" inputmode=%q"`
	Is        CustomElement `html:" is=%q"`

	ItemID       ItemID       `html:" itemid=%q"`
	ItemProperty ItemProperty `html:" itemprop=%q"`
	ItemRef      []ItemID     `html:" itemref=%q"`
	ItemScope    bool         `html:" itemscope"`
	ItemType     url.String   `html:" itemtype=%q"`

	Language string `html:" lang=%q"`
	Nonce    string `html:" nonce=%q"`
	Part     []Part `html:" part=%q"`

	Popover    bool     `html:" popover"`
	Slot       SlotName `html:" slot=%q"`
	SpellCheck Bool     `html:" spellcheck"`

	Style css.String `html:" style=%q"`

	TabIndex              int    `html:" tabindex=\"%d\""`
	Tooltip               string `html:" title=%q"`
	Translate             YesNo  `html:" translate=%q"`
	VirtualKeyboardPolicy string `html:" virtualkeyboardpolicy=%q"`
}

type Text string

func (Text) HTML5() {}

func InnerText(text string) Nodes {
	return Nodes{Text(text)}
}

func (t Text) Format(w fmt.State, verb rune) {
	fmt.Fprintf(w, html.EscapeString(string(t)))
}

type PermissionsPolicy string
