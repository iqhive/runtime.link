/*
Package cmdl provides a command-line interface linker for runtime.link.

# Function Tags

Tags can be added  to functions to indicate how arguments should
be mapped to command-line arguments. Each space seperated component
will be passed as a seperate argument to the command-line. A component
may contain a format placeholder ('%v' or '%[n]v').

# Struct Tags

Tags can be added to fields to indicate how they should be packed and
unpacked from [os.Args]. Rules behave simarly to function tags,
command line parameters are included by default unless they are a bool
without a format parameter or are flagged as 'omitempty'. Field tags can
additionally specify one of the subsequent flags:

  - 'env'
    variable.

  - 'dir'
    sets the working directory.

  - 'invert'
    bool (and the behaviour of omitempty).

The documentation of a field tag will be used for the help text. If a
field is a [io.Reader] it will be passed to stdin, [io.Writer] will be
passed to stdout by default unless the field is tagged with `cmdl:",stderr"`.
*/
package cmdl

import (
	"fmt"
	"os/exec"
	"reflect"

	"runtime.link/api"
)

// API implements the [api.Linker] interface.
var API api.Linker[string, *exec.Cmd] = linker{}

type linker struct{}

// Link implements the [api.Linker] interface.
func (linker) Link(structure api.Structure, cmd string, client *exec.Cmd) error {
	_, err := exec.LookPath(cmd)
	if err == nil {
		structure.Host = reflect.StructTag(fmt.Sprintf(`cmd:"%v"`, cmd))
	}
	linkStructure(structure)
	return nil
}
