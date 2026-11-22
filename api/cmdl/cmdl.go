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
of its parameter, and arguments left over are ignored. A word that comes
after a struct of flags is looked for among the arguments that follow them,
so of commands that are told apart by such a word, as 'tool %v put %v' and
'tool %v del %v' are, the one whose words are all there is the one that runs.
If that is so for more than one, the first declared of them runs, so a
command that takes any word in that place, as 'tool %v %v %v' does, runs for
the word of a command declared after it.

When linked, a slice parameter is passed as an argument for each element, so
a []byte passes a number for each byte. When running as a command a []byte is
read as the text of one argument, so a link and a program do not agree on a
[]byte parameter: 'write %v %v' called with "hi" and "name" passes
'write 104 105 name', which a program reads as "104" and "105".

# Struct Tags

Tags can be added to fields to indicate how they should be packed and
unpacked from [os.Args]. Rules behave simarly to function tags,
command line parameters are included by default unless they are a bool
whose tag has no '%v', such as '--on' or '--on=%t', or are flagged as
'omitempty'/'omitzero'. A field tag takes any verb of package fmt, such as
'--hex=%x', so as in fmt '%%' is a literal '%', and any other '%' stands for
itself. A value prints as fmt prints it, so a time.Time prints as
[time.Time.String] does, except that a pointer prints what it points to,
and a nil pointer leaves out a flag, env or dir that would print it. A slice
prints whole into its one flag, as '--tag=[a b]', and a []byte under '%v' as
a number for each byte, '--data=[104 105]'. A word with more than one verb
prints the value for each of them. Field tags can additionally specify one
of the subsequent flags:

  - 'env'
    variable.

  - 'dir'
    sets the working directory.

  - 'invert'
    bool (and the behaviour of omitempty). When linked, a pointer to a bool
    is passed as what it points to, not inverted. When running as a command,
    the bool that a pointer leads to is inverted like any other, so such a
    flag does not read back what a link passed for it.

When running as a command, a value is read the way fmt reads it, and all
of it: '--count=12abc' is an error, '--count= 12' is 12. A verb reads by its
letter, without the flags, width and precision that it prints with, so what
'--price=%.2f' or '--serial=%05d' prints is read back, and an integer that
'%#x', '%#o', '%#b' or '%O' printed with the prefix of its base is read with
it. A time.Time is
read in RFC 3339 or as [time.Time.String] prints it. Flags end at the first
argument that is not one. A string flag whose argument does not fit the
text after its verb takes everything after the text before its first '%'.
A time.Duration is also read as it prints, such as '1.5s'. A pointer to a bool,
a string or a group of flags is set to what its flag says, and a slice takes a
value each time its flag is repeated. A slice of numbers or bools also takes
the list that a link prints into one flag, such as '-p [80 443]'. A slice of
strings and a []byte do not: they read '--tag=[a b]' as the one string
'[a b]' and '--data=[104 105]' as that text, which is not what a link was
given. A parameter that is a pointer to a
struct of flags is nil unless an argument set one of them. A variadic parameter
of any type takes an argument for each of its values, each the name of a file
to open if it is an [io.Reader] or another interface that a file fits, and a
[]byte is the text of its argument, under '%v' and under '%s'.

The help text shows a struct of flags as '[--flag ...]', without the
documentation of its fields. If a
field is a [io.Reader] it will be passed to stdin, [io.Writer] will be
passed to stdout by default unless the field is tagged with `cmdl:",stderr"`.
A field that is both goes to stdin. These only apply to a field with no name
in its tag that is set. Any other field with no name in its tag, including an
unset reader or writer, passes an empty argument (a bool only when true).

When linked, a value with a text form, as an [encoding.TextMarshaler], is
passed as that text where it is an argument by itself, a slice such as a
[net.IP] and a pointer such as a *[math/big.Int] included. A struct of flags under
a name with no verb is a group: its flags follow the name.

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
per line, so a []byte prints a number per byte. A result, or an element of
one, prints as fmt prints it, so a type with a String method prints what that
returns, which is not what a link to the same function reads back.

A function streams when its first result is a chan []byte, or its last result
before any error is a chan error, and its other results are zero. A chan
[]byte receives lines of stdout as the command runs, and a chan error
receives lines of stderr followed by how the command ended, if it failed.
Both close however the command ends, including when it cannot start. Read
them until they close, or cancel the context. A function with neither a chan
error nor an error result cannot be told that the command failed. Output
that no channel receives is discarded, except the stderr of a function with
no error result, which goes to os.Stderr. The command is never kept waiting
for its chan error to be read: lines of stderr that have not been read are
kept, however many there are, until they are read or the context is
cancelled.

The error of a call that failed reads what the command wrote to stderr, if it
wrote anything, and how the call ended is behind it, for [errors.As] to find
the [os/exec.ExitError] with the exit status.

When running as a command, a function whose results are a chan []byte, a chan
error or both is read until they close. Each value of a chan []byte is
written as a line of stdout when it is sent, and each error of a chan error as
a line of stderr, the last of them being the error that the command fails
with. A result that cannot be written is an error.

On linux, a cancelled call kills the process group of the command, which has
one of its own, unless the command is given the terminal that controls its
caller as its stdin, stdout or stderr. It then stays in the process group of
its caller, so that it can read the terminal and an interrupt typed there
reaches it, and cancelling its call kills the command alone.

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
