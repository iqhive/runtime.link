/*
Package cmdl provides a command-line interface linker for runtime.link.

# Function Tags

Tags can be added  to functions to indicate how arguments should
be mapped to command-line arguments. Each space seperated component
will be passed as a seperate argument to the command-line, so a doubled
space passes an empty one. A component that starts with '%' or '{' is a
placeholder, '%v', '%[n]v' or the '{name}' of a parameter, with any brace at
either end trimmed off. Any other component that starts with '%' or '{' is
passed as it is, such as '%s' or '{}', except that '%%v' passes '%v'. The
rest are passed as they are written, so 'date +%Y' passes '+%Y' and '+%%v'
passes '+%%v'.

A placeholder can also be inside a component, such as '--name=%v' or
'x%v.txt', but only when the parameters fill every placeholder of the tag
that way. Otherwise such a component is passed as it is written, as it
always was, so the tag '--fmt=%v %v' of a function with one parameter
passes '--fmt=%v' and then the parameter.

When running as a command, a value inside a component ends at the first
match of the text after its placeholder, and an argument that does not fit
around it leaves the parameter at its zero value. Placeholders that touch,
such as '%v%v', can only be passed to a linked command, there is no telling
where the first one would end.

When running as a command, a tag ends at its first comma, and the result is
written as JSON if the tag contains ',json' anywhere. The word the comma is
in matches with or without what follows the comma, so 'version,json' runs
for both 'version' and 'version,json'. When linked, a trailing ',json' reads
the output as JSON only if it follows a placeholder, or if the results could
not be read without it (see Results). Otherwise it is part of the last
argument, as in 'list,json' for a string result. ' | json' at the end of a
tag always reads the output as JSON.

When running as a command, a word of the command that is missing or
different means it is not that command. A missing value is the zero value
of its parameter, and arguments left over are ignored.

# Struct Tags

Tags can be added to fields to indicate how they should be packed and
unpacked from [os.Args]. Rules behave simarly to function tags,
command line parameters are included by default unless they are a bool
whose tag has no '%v', such as '--on' or '--on=%t', or are flagged as
'omitempty'/'omitzero'. A field tag takes any verb of package fmt, such as
'--hex=%x', so as in fmt '%%' is a literal '%', and any other '%' stands for
itself. A value prints as fmt prints it, so a time.Time prints as
[time.Time.String] does, except that a pointer prints what it points to,
and a nil pointer leaves out a flag, env or dir that would print it. Field
tags can additionally specify one of the subsequent flags:

  - 'env'
    variable.

  - 'dir'
    sets the working directory.

  - 'invert'
    bool (and the behaviour of omitempty).

When running as a command, a value is read the way fmt reads it, and all
of it: '--count=12abc' is an error, '--count= 12' is 12. A time.Time is
read in RFC 3339 or as [time.Time.String] prints it. Flags end at the first
argument that is not one. A string flag whose argument does not fit the
text after its verb takes everything after the text before its first '%'.

The documentation of a field tag will be used for the help text. If a
field is a [io.Reader] it will be passed to stdin, [io.Writer] will be
passed to stdout by default unless the field is tagged with `cmdl:",stderr"`.
A field that is both goes to stdin. These only apply to a field with no name
in its tag that is set. Any other field with no name in its tag, including an
unset reader or writer, passes an empty argument (a bool only when true).

When linked, an embedded struct passes its flags as those of the struct that
embeds it, whatever its tag if it is exported, and unless it is tagged '-' if
it is not. An embedded pointer does so when it is set and has no tag,
otherwise it is a field like any other. When running as a command, the flags
of an embedded struct or pointer belong to the struct that embeds it unless
it is tagged '-', and a nil embedded pointer is filled in, except an
unexported one, which cannot be set.

# Results

When calling a command, a string receives its stdout without the trailing
newline, a []string its lines, a []byte exactly what it wrote, and an int32 the
number it printed. Any other slice is empty if no line of output ended.
Anything else has to be read as JSON. A function with results that cannot be
filled in runs the command, and then fails with an error that says
'not implemented'. When running as a command, a slice is printed an element
per line, so a []byte prints a number per byte.

A function streams when its first result is a chan []byte, or its last result
before any error is a chan error, and its other results are zero. A chan
[]byte receives lines of stdout as the command runs, and a chan error
receives lines of stderr followed by how the command ended, if it failed.
Both close however the command ends, including when it cannot start. Read
them until they close, or cancel the context. A function with neither a chan
error nor an error result cannot be told that the command failed. Output
that no channel receives is discarded, except the stderr of a function with
no error result, which goes to os.Stderr.

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
