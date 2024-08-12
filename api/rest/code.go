package rest

import (
	"bytes"
	"fmt"

	"runtime.link/api/internal/oas"
)

func sdkFor(docs oas.Document) ([]byte, error) {
	var buf bytes.Buffer
	fmt.Fprintln(&buf, `/**`)
	fmt.Fprintf(&buf, ` * Returns a new API client for the %s API`+"\n", docs.Information.Title)
	fmt.Fprintln(&buf, ` *`)
	fmt.Fprintln(&buf, ` * @param {string} host url for the API`)
	fmt.Fprintln(&buf, ` * @param {function} fetch function defaults to window.fetch`)
	fmt.Fprintln(&buf, ` * @returns {Object} API client`)
	fmt.Fprintln(&buf, ` */`)
	fmt.Fprintln(&buf, "export function API(host, fetch) {")
	fmt.Fprint(&buf, "\t")
	fmt.Fprintln(&buf, "return {")

	fmt.Fprint(&buf, "\t")
	fmt.Fprintln(&buf, "}")
	fmt.Fprintln(&buf, "}")
	return buf.Bytes(), nil
}
