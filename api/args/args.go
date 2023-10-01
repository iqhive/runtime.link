/*
Package args provides a command-line interface layer for runtime.link.

# Function Tags
Tags can be added  to functions to indicate how their arguments should
be mapped to command-line arguments. Each space seperated component
will be passed as a seperate argument to the command-line. A component
can either be a literal string or a format placeholder ('%v' or '%[n]v').

# Field Tags
Tags can be added to fields to indicate how they should be transformed
into a command-line argument. Rules behave simarly to function tags,
command line parameters are included by default unless they are a bool
without a format parameter or are flagged as 'omitempty'. Field tags can
additionally specify one of the subsequent flags:

  - 'env'
    variable.

  - 'dir'
    working directory.

  - 'invert'
    bool (and the behaviour of omitempty).

The documentation of a field tag will be used for the help text. If a
field is a [io.Reader] it will be passed to stdin, [io.Writer] will be
passed to stdout by default unless the field is tagged with `args:"stdout"`.
*/
package args

import (
	"fmt"
	"os/exec"
	"reflect"

	"runtime.link/api"
)

// API implements the [api.Linker] interface.
var API transport

type transport struct{}

// Link implements the [api.Linker] interface.
func (transport) Link(structure api.Structure, cmd string, client *exec.Cmd) error {
	_, err := exec.LookPath(cmd)
	if err == nil {
		structure.Host = reflect.StructTag(fmt.Sprintf(`exec:"%v"`, cmd))
	}
	Link(structure)
	return nil
}
