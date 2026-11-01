/*
Package cmdl provides a command-line interface linker for runtime.link.

# Function Tags

Tags can be added  to functions to indicate how arguments should
be mapped to command-line arguments. Each space seperated component
will be passed as a seperate argument to the command-line. A component
may contain format placeholders ('%v' or '%[n]v'), anywhere in it, such as
'--name=%v' or '%v.txt'. No other '%' means anything, so 'date +%Y' passes
'+%Y' as it is, and '%%v' passes a literal '%v'.

When running as a command, a value ends at the first match of the text after
its placeholder. Placeholders that touch, such as '%v%v', can only be passed
to a linked command, there is no telling where the first one would end.

A tag that ends in exactly ',json' writes the result as JSON when running as a
command, and reads the output as JSON when linked. Any other comma belongs to
an argument, such as in '-o pid,comm'. An argument that really does end in
',json' has to be passed through a placeholder.

# Struct Tags

Tags can be added to fields to indicate how they should be packed and
unpacked from [os.Args]. Rules behave simarly to function tags,
command line parameters are included by default unless they are a bool
without a format parameter or are flagged as 'omitempty'/'omitzero'. A field
tag takes any verb of package fmt, such as '--hex=%x', so as in fmt '%%' is a
literal '%', and any other '%' stands for itself. Field tags can additionally
specify one of the subsequent flags:

  - 'env'
    variable.

  - 'dir'
    sets the working directory.

  - 'invert'
    bool (and the behaviour of omitempty).

The documentation of a field tag will be used for the help text. If a
field is a [io.Reader] it will be passed to stdin, [io.Writer] will be
passed to stdout by default unless the field is tagged with `cmdl:",stderr"`.
A field that is both goes to stdin. These only apply to a field with no name
in its tag, one with a name is an argument like any other. Any other field
with no name is a positional argument when it is set, unless it is a bool.

The flags of an embedded struct belong to the struct that embeds it, whether
or not it is embedded by pointer. A nil embedded pointer is filled in when
running as a command, except an unexported one, which cannot be set.

# Results

When calling a command, a string receives its stdout without the trailing
newline, a []string its lines, a []byte exactly what it wrote, and an int32 the
number it printed. Anything else has to be read as JSON. A function with
results that cannot be filled in fails when called, before anything runs.

A chan []byte receives lines of stdout as the command runs, and a chan error
receives lines of stderr followed by how the command ended, if it failed.
Both close however the command ends, including when it cannot start. Read
them until they close, or cancel the context. A function with neither a chan
error nor an error result cannot be told that the command failed.

Missing-command help has a usage line per function, in the order its arguments
are typed (for example `pos [--flag ...] <name>` for the tag 'pos %[2]v %[1]v').
When the API implements [api.WithSource] they are named from
[api.Function.Args], otherwise they read `<value>`.
*/
package cmdl

import (
	"os/exec"

	"github.com/iqhive/runtime.link/api"
)

// API implements the [api.Linker] interface.
var API api.Linker[string, *exec.Cmd] = linker{}

type linker struct{}

// Link implements the [api.Linker] interface.
func (linker) Link(structure api.Structure, cmd string, client *exec.Cmd) error {
	linkStructure(structure, cmd, "")
	return nil
}
