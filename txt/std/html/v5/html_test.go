package html_test

import (
	"fmt"
	"testing"

	"runtime.link/txt/std/html/v5"
)

func TestDOM(t *testing.T) {
	fmt.Println(html.Value(html.Document{
		Tree: html.Nodes{
			html.Head{
				Tree: html.Nodes{
					html.Title{
						Text: "Hello, world!",
					},
				},
			},
			html.Body{
				With: html.Attributes{
					Style: "background-color: black;",
				},
				Tree: html.Nodes{
					html.Paragraph{
						Tree: html.InnerText("Hello, world!"),
					},
				},
			},
		},
	}))
}
