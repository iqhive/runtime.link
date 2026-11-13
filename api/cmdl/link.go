package cmdl

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/xray"
)

// drainDelay bounds how long what a command leaves behind can hold up its
// caller once the command itself has exited: a descendant that inherited
// its output can keep a pipe open indefinitely, and a Read of a caller's
// [io.Reader] cannot be interrupted at all.
const drainDelay = 5 * time.Second

type cmdInput struct {
	args []string
	env  []string
	wd   string

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func (input *cmdInput) add(val reflect.Value) error {
	switch val.Kind() {
	case reflect.Struct:
		rtype := val.Type()
		if rtype.Implements(reflect.TypeFor[encoding.TextMarshaler]()) && val.CanInterface() {
			data, err := val.Interface().(encoding.TextMarshaler).MarshalText()
			if err != nil {
				return xray.New(err)
			}
			input.args = append(input.args, string(data))
			return nil
		}
		for i := 0; i < rtype.NumField(); i++ {
			field := rtype.Field(i)
			if isFlattened(field, val.Field(i)) {
				if err := input.add(val.Field(i)); err != nil {
					return xray.New(err)
				}
				continue
			}
			if !field.IsExported() {
				continue
			}
			exec := field.Tag.Get("cmdl")
			if exec == "-" {
				continue
			}
			if err := input.addField(val.Field(i), exec); err != nil {
				return xray.New(err)
			}
		}
	case reflect.Slice:
		// A slice with a text form of its own, such as a net.IP, is one
		// value. Its elements were once passed as an argument each.
		if text, ok, err := ownText(val); ok || err != nil {
			if err != nil {
				return xray.New(err)
			}
			input.args = append(input.args, text)
			return nil
		}
		for i := 0; i < val.Len(); i++ {
			if err := input.add(val.Index(i)); err != nil {
				return xray.New(err)
			}
		}
	case reflect.Pointer:
		if val.IsNil() {
			return nil
		}
		// A struct whose text form is only had through its pointer, such as
		// a *big.Int, once passed nothing at all, as it has nothing exported.
		if elem := val.Type().Elem(); elem.Kind() == reflect.Struct && !elem.Implements(reflect.TypeFor[encoding.TextMarshaler]()) && !hasExportedField(elem) && !isFlagStruct(elem) {
			if text, ok, err := ownText(val); ok || err != nil {
				if err != nil {
					return xray.New(err)
				}
				input.args = append(input.args, text)
				return nil
			}
		}
		if err := input.add(val.Elem()); err != nil {
			return xray.New(err)
		}
	default:
		input.args = append(input.args, fmt.Sprint(val.Interface()))
	}
	return nil
}

func hasExportedField(rtype reflect.Type) bool {
	for i := 0; i < rtype.NumField(); i++ {
		if rtype.Field(i).IsExported() {
			return true
		}
	}
	return false
}

// ownText returns the text of val if it is an [encoding.TextMarshaler].
func ownText(val reflect.Value) (text string, ok bool, err error) {
	if !val.CanInterface() {
		return "", false, nil
	}
	marshaler, ok := val.Interface().(encoding.TextMarshaler)
	if !ok {
		return "", false, nil
	}
	data, err := marshaler.MarshalText()
	return string(data), true, err
}

// isFlattened reports whether the flags of an embedded struct are passed as
// the flags of the struct that embeds it. Every exported embedded struct
// always was, whatever its tag. The rest were read as fields, which lost the
// flags of an unexported one, and of a set pointer with no tag, that passed
// an empty argument instead.
func isFlattened(field reflect.StructField, value reflect.Value) bool {
	if !field.Anonymous {
		return false
	}
	tag := field.Tag.Get("cmdl")
	switch field.Type.Kind() {
	case reflect.Struct:
		return field.IsExported() || (tag != "-" && isFlagStruct(field.Type))
	case reflect.Pointer:
		return tag == "" && !value.IsNil() && isFlagStruct(field.Type.Elem())
	}
	return false
}

func (input *cmdInput) addField(value reflect.Value, exec string) error {
	name, _, _ := strings.Cut(exec, ",")
	if name == "" && input.addStream(value, exec) {
		return nil
	}
	if strings.Contains(exec, ",invert") && value.Kind() == reflect.Bool {
		value = reflect.ValueOf(!value.Bool())
	}
	// Only a '%v' has ever kept a false bool, so '--on=%t' leaves it out.
	omitBooleanFlag := !strings.Contains(exec, "%v") && value.Kind() == reflect.Bool
	if (strings.Contains(exec, ",omitempty") || strings.Contains(exec, ",omitzero") || omitBooleanFlag) && value.IsZero() {
		return nil
	}
	var (
		arg, present = flagValue(value)
		isEnv        = strings.Contains(exec, ",env")
		isDir        = strings.Contains(exec, ",dir")
	)
	if !present && (isEnv || isDir || hasVerb(name)) {
		return nil
	}
	switch {
	case isEnv:
		input.env = append(input.env, fmt.Sprintf("%s=%v", name, arg))
	case isDir:
		input.wd = fmt.Sprint(arg)
	default:
		for _, part := range strings.Split(name, " ") {
			format, verbs := flagFormat(part)
			if verbs == 0 {
				input.args = append(input.args, unescapePercent(format))
				continue
			}
			input.args = append(input.args, fmt.Sprintf(format, arg))
		}
		// The flags of a group follow the word that names it, where a
		// program reads them. They were once dropped.
		if group := reflect.ValueOf(arg); present && name != "" && !hasVerb(name) && isFlagStruct(group.Type()) {
			return input.add(group)
		}
	}
	return nil
}

func hasVerb(name string) bool {
	_, verbs := flagFormat(name)
	return verbs > 0
}

// addStream is only for a field with no name in its tag, because a named one
// asks to be an argument whatever its type. An unset one is left to be the
// empty argument that any field with no name passes.
func (input *cmdInput) addStream(val reflect.Value, tag string) bool {
	isReader := val.Type().Implements(reflect.TypeFor[io.Reader]())
	isWriter := val.Type().Implements(reflect.TypeFor[io.Writer]())
	if (!isReader && !isWriter) || val.IsZero() {
		return false
	}
	switch {
	case isWriter && strings.Contains(tag, ",stderr"):
		input.stderr = val.Interface().(io.Writer)
	case isReader:
		input.stdin = val.Interface().(io.Reader)
	default:
		input.stdout = val.Interface().(io.Writer)
	}
	return true
}

// flagValue returns what a flag prints for val, which is what package fmt
// prints for it, except that a pointer prints what it points to rather than
// its address. present is false when a nil leaves nothing to print.
func flagValue(val reflect.Value) (arg any, present bool) {
	for val.Kind() == reflect.Pointer || val.Kind() == reflect.Interface {
		if val.IsNil() {
			return nil, false
		}
		val = val.Elem()
	}
	return val.Interface(), true
}

// flagVerbSize differs from placeholderSize because a field tag takes any verb
// of package fmt, as in '--hex=%x'. There is no space flag here, a tag is split
// on spaces and '100% done' is not a verb.
func flagVerbSize(text string) int {
	rest, ok := strings.CutPrefix(text, "%")
	if !ok {
		return 0
	}
	rest = strings.TrimLeft(rest, "+-#0123456789")
	if precision, ok := strings.CutPrefix(rest, "."); ok {
		rest = strings.TrimLeft(precision, "0123456789")
	}
	if indexed, ok := strings.CutPrefix(rest, "["); ok {
		index, after, ok := strings.Cut(indexed, "]")
		if !ok || index == "" || strings.Trim(index, "0123456789") != "" {
			return 0
		}
		rest = after
	}
	if rest == "" || !strings.ContainsAny(rest[:1], "vTtbcdoOqxXUeEfFgGsp") {
		return 0
	}
	return len(text) - len(rest) + 1
}

// flagFormat makes a field tag safe to give to package fmt, which would print
// a '%' that is not a verb as an error in the middle of an argument. As in fmt,
// '%%' is a literal '%', which is how a tag spells out a '+%d' that looks like
// a verb but is not one.
func flagFormat(name string) (format string, verbs int) {
	var safe strings.Builder
	for i := 0; i < len(name); {
		text := name[i:]
		size := flagVerbSize(text)
		switch {
		case size > 0:
			safe.WriteString(text[:size])
			verbs++
			i += size
		case strings.HasPrefix(text, "%%"):
			safe.WriteString("%%")
			i += len("%%")
		case text[0] == '%':
			safe.WriteString("%%")
			i++
		default:
			safe.WriteByte(text[0])
			i++
		}
	}
	return safe.String(), verbs
}

func unescapePercent(format string) string {
	return strings.ReplaceAll(format, "%%", "%")
}

// splitFlag cuts a field tag around its first verb, for reading an argument
// back, where there is only one value to find.
func splitFlag(name string) (prefix, verb, suffix string) {
	format, _ := flagFormat(name)
	for i := 0; i < len(format); {
		text := format[i:]
		if strings.HasPrefix(text, "%%") {
			i += len("%%")
			continue
		}
		if size := flagVerbSize(text); size > 0 {
			return unescapePercent(format[:i]), text[:size], unescapePercent(text[size:])
		}
		i++
	}
	return unescapePercent(format), "", ""
}

// tagWord is a word of a function tag, split around the placeholders in it.
// literals[i] comes before formats[i] and the last literal after them all, so
// a word with no placeholders is a single literal.
type tagWord struct {
	literals []string
	formats  []string
}

// placeholderSize is what tells a placeholder from any other use of '%', such
// as the '+%Y' of date: only what [api.ArgumentScanner] can consume counts.
func placeholderSize(text string) int {
	if strings.HasPrefix(text, "%v") {
		return len("%v")
	}
	indexed, ok := strings.CutPrefix(text, "%[")
	if !ok {
		return 0
	}
	index, _, ok := strings.Cut(indexed, "]v")
	if !ok || index == "" || strings.Trim(index, "0123456789") != "" {
		return 0
	}
	return len("%[") + len(index) + len("]v")
}

func isNamedPlaceholder(word string) bool {
	return len(word) > len("{}") && strings.HasPrefix(word, "{") && strings.HasSuffix(word, "}")
}

// parseWord is shared so that a tag means the same to a program as to a link.
// Only '%%' in front of a placeholder is an escape, which is what keeps a tag
// able to pass a real '%v' in a word that starts with '%'. A word with no
// placeholder that asWritten says was always passed as it was written still
// is, '%%' and all.
func parseWord(component string, asWritten bool) tagWord {
	if isNamedPlaceholder(component) {
		return tagWord{literals: []string{"", ""}, formats: []string{strings.Trim(component, "{}")}}
	}
	var (
		word    tagWord
		literal strings.Builder
	)
	for i := 0; i < len(component); {
		text := component[i:]
		size := placeholderSize(text)
		escaped := 0
		if strings.HasPrefix(text, "%%") {
			escaped = placeholderSize(text[1:])
		}
		switch {
		case size > 0:
			word.literals = append(word.literals, literal.String())
			word.formats = append(word.formats, text[:size])
			literal.Reset()
			i += size
		case escaped > 0:
			literal.WriteString(text[1 : 1+escaped])
			i += 1 + escaped
		default:
			literal.WriteByte(component[i])
			i++
		}
	}
	word.literals = append(word.literals, literal.String())
	if word.isLiteral() && asWritten {
		return literalWord(component)
	}
	return word
}

func literalWord(text string) tagWord { return tagWord{literals: []string{text}} }

func (word tagWord) isLiteral() bool { return len(word.formats) == 0 }

func (word tagWord) isWhole() bool {
	return len(word.formats) == 1 && word.literals[0] == "" && word.literals[1] == ""
}

// keepWrittenIfUnfilled reads a placeholder in a word that asWritten says was
// always taken as it was written, such as '--name=%v', as a placeholder only
// when the parameters can fill every placeholder of the tag that way. A tag
// written for the old reading, with no parameter left for such a word, is
// still read the old way.
func keepWrittenIfUnfilled(words []tagWord, components []string, scanner api.ArgumentScanner, asWritten func(string) bool) []tagWord {
	if fills(words, scanner) {
		return words
	}
	for i, word := range words {
		if !word.isLiteral() && asWritten(components[i]) {
			words[i] = literalWord(components[i])
		}
	}
	return words
}

// fills scans a copy of scanner, so the caller's is left where it was.
func fills(words []tagWord, scanner api.ArgumentScanner) bool {
	for _, word := range words {
		for _, format := range word.formats {
			if _, err := scanner.Scan(format); err != nil {
				return false
			}
		}
	}
	return true
}

// isWrittenForLink is whether a link has always passed a word as it was
// written. Any word that starts with '%' or '{' was scanned for instead.
func isWrittenForLink(component string) bool {
	return !strings.HasPrefix(component, "%") && !strings.HasPrefix(component, "{")
}

// oldPlaceholder keeps the placeholders that only the old reading of a link
// tag had: any word that starts with '%' or '{', with every brace at either end
// trimmed off, was scanned for, so '{name' and '%v}' were placeholders too.
// Only those that could be scanned ever worked.
func oldPlaceholder(component string, scanner api.ArgumentScanner) (string, bool) {
	if isWrittenForLink(component) {
		return "", false
	}
	format := strings.Trim(component, "{}")
	if format != "" && placeholderSize(format) == len(format) {
		return format, true
	}
	if format == "" || strings.Contains(format, "%") {
		return "", false
	}
	_, err := scanner.Scan(format)
	return format, err == nil
}

// linkWords splits a tag on every space, so a doubled space is an empty
// argument, as it always was.
func linkWords(tag string, scanner api.ArgumentScanner) []tagWord {
	if tag == "" {
		return nil
	}
	components := strings.Split(tag, " ")
	words := make([]tagWord, len(components))
	for i, component := range components {
		if format, ok := oldPlaceholder(component, scanner); ok {
			words[i] = tagWord{literals: []string{"", ""}, formats: []string{format}}
			continue
		}
		words[i] = parseWord(component, isWrittenForLink(component))
	}
	return keepWrittenIfUnfilled(words, components, scanner, isWrittenForLink)
}

// render takes every placeholder's argument even once one turns out to be nil,
// or the arguments after this word would shift.
func (word tagWord) render(scanner *api.ArgumentScanner) (text string, present bool, err error) {
	var rendered strings.Builder
	present = true
	for i, format := range word.formats {
		val, err := scanner.Scan(format)
		if err != nil {
			return "", false, xray.New(err)
		}
		arg, ok, err := wordValue(val)
		if err != nil {
			return "", false, xray.New(err)
		}
		present = present && ok
		rendered.WriteString(word.literals[i])
		fmt.Fprint(&rendered, arg)
	}
	rendered.WriteString(word.literals[len(word.formats)])
	return rendered.String(), present, nil
}

// wordValue returns what a placeholder inside a word prints for val: what a
// pointer points to, not its address, and a marshaler's text, the form
// [System.Run] parses back. present is false when a nil leaves nothing to
// print.
func wordValue(val reflect.Value) (arg any, present bool, err error) {
	for {
		isIndirect := val.Kind() == reflect.Pointer || val.Kind() == reflect.Interface
		if isIndirect && val.IsNil() {
			return nil, false, nil
		}
		if marshaler, ok := val.Interface().(encoding.TextMarshaler); ok {
			data, err := marshaler.MarshalText()
			if err != nil {
				return nil, false, err
			}
			return string(data), true, nil
		}
		if !isIndirect {
			return val.Interface(), true, nil
		}
		val = val.Elem()
	}
}

func newInput(tag string, scanner api.ArgumentScanner) (cmdInput, error) {
	var input cmdInput
	for _, word := range linkWords(tag, scanner) {
		switch {
		case word.isLiteral():
			input.args = append(input.args, word.literals[0])
		case word.isWhole():
			val, err := scanner.Scan(word.formats[0])
			if err != nil {
				return input, xray.New(err)
			}
			if err := input.add(val); err != nil {
				return input, xray.New(err)
			}
		default:
			text, present, err := word.render(&scanner)
			if err != nil {
				return input, err
			}
			if present {
				input.args = append(input.args, text)
			}
		}
	}
	return input, nil
}

// cutJSONOption reads a trailing ',json' as the option to read the output as
// JSON only where it never worked as part of the last argument: after a
// placeholder, which could not be scanned with it, or for results that cannot
// be read without it. Anywhere else it is passed on, as it always was.
func cutJSONOption(fn api.Function, tag string) (string, bool) {
	cut, ok := strings.CutSuffix(tag, ",json")
	if !ok {
		return tag, false
	}
	last := cut[strings.LastIndex(cut, " ")+1:]
	if !isWrittenForLink(last) || !readsWithoutJSON(fn) {
		return cut, true
	}
	return tag, false
}

func linkStructure(spec api.Structure, cmd string, prefix string) {
	if _, err := exec.LookPath(cmd); err != nil {
		missing := fmt.Errorf("cannot find program '%s': %w", cmd, err)
		for fn := range spec.Iter() {
			fn.Make(func(context.Context, []reflect.Value) ([]reflect.Value, error) {
				return failedCall(fn, missing)
			})
		}
		return
	}
	for _, fn := range spec.Functions {
		link(cmd, fn, prefix)
	}
	for _, section := range spec.Namespace {
		section.Host = spec.Host
		linkStructure(section, cmd, joinPrefix(prefix, section.Tags.Get("cmdl")))
	}
}

// joinPrefix leaves the tag of a top-level namespace exactly as it is, the
// only one whose prefix was ever passed on.
func joinPrefix(prefix, tag string) string {
	if prefix == "" || tag == "" {
		return prefix + tag
	}
	return prefix + " " + tag
}

func link(cmd string, fn api.Function, prefix string) {
	if prefix != "" {
		prefix += " "
	}
	tag := prefix + string(fn.Tags.Get("cmdl"))
	if cmd == "" {
		cmd, tag, _ = strings.Cut(tag, " ")
	}
	tag, isJSON := cutJSONOption(fn, tag)
	if strings.HasSuffix(tag, " | json") {
		tag = strings.TrimSuffix(tag, " | json")
		isJSON = true
	}
	fn.Make(func(ctx context.Context, args []reflect.Value) ([]reflect.Value, error) {
		execArgs, err := newInput(tag, api.NewNamedArgumentScanner(args, fn.Args))
		if err != nil {
			return failedCall(fn, err)
		}
		if os.Getenv("DEBUG_CMD") != "" {
			fmt.Printf("%s {%v %v %v}\n", cmd, execArgs.args, execArgs.env, execArgs.wd)
		}
		cmd := exec.CommandContext(ctx, cmd, execArgs.args...)
		cmd.Env = append(os.Environ(), execArgs.env...)
		cmd.Dir = execArgs.wd
		cmd.WaitDelay = drainDelay

		if lines, errs := streamedResults(fn); lines || errs {
			return stream(ctx, fn, cmd, execArgs)
		}

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		var captureStdout, captureStderr io.Writer
		if fn.NumOut() > 0 {
			captureStdout = &stdout
		}
		if hasErrorResult(fn) {
			captureStderr = &stderr
		}
		cmd.Stdout = outputTo(captureStdout, execArgs.stdout, os.Stdout)
		cmd.Stderr = outputTo(captureStderr, execArgs.stderr, os.Stderr)

		wait, err := start(cmd, execArgs.stdin)
		if err == nil {
			err = wait()
		}
		if errors.Is(err, exec.ErrWaitDelay) {
			// A descendant still holds the output open, but the command
			// itself exited cleanly, and everything it wrote has been read.
			// The call used to return the moment the command exited, with
			// no error, so it still does, only drainDelay later.
			err = nil
		}
		if err != nil {
			if text := stderr.String(); strings.TrimSpace(text) != "" {
				return nil, commandError{stderr: text, cause: xray.New(err)}
			}
			return nil, xray.New(err)
		}
		if fn.NumOut() == 0 {
			return nil, nil
		}
		if fn.NumOut() > 1 {
			return nil, fmt.Errorf("cmdl: return type %v: not implemented", fn.Type.Out(0))
		}
		result, err := decodeResult(fn.Type.Out(0), &stdout, isJSON)
		if err != nil {
			return nil, err
		}
		return []reflect.Value{result}, nil
	})
}

func hasErrorResult(fn api.Function) bool { return fn.NumOut() != fn.Type.NumOut() }

// commandError reports a failed call in the words of the command, what it
// wrote to stderr, which say more than its exit status does. The status, or
// whatever else ended the call, is what it wraps, so that [errors.As] still
// finds an [exec.ExitError].
type commandError struct {
	stderr string
	cause  error
}

func (err commandError) Error() string { return err.stderr }
func (err commandError) Unwrap() error { return err.cause }

func readsWithoutJSON(fn api.Function) bool {
	lines, errs := streamedResults(fn)
	switch {
	case lines || errs || fn.NumOut() == 0:
		return true
	case fn.NumOut() == 1:
		return readsAsText(fn.Type.Out(0))
	}
	return false
}

// readsAsText lists what decodeResult can build from output that is not JSON.
func readsAsText(rtype reflect.Type) bool {
	switch rtype.Kind() {
	case reflect.String, reflect.Int32:
		return true
	case reflect.Slice:
		return rtype.Elem().Kind() == reflect.Uint8 || rtype.Elem().Kind() == reflect.String
	}
	return false
}

// decodeResult turns the output of a finished command into its result. A
// []byte is the output exactly as written, since it may not even be text.
func decodeResult(rtype reflect.Type, stdout *bytes.Buffer, isJSON bool) (reflect.Value, error) {
	value := reflect.New(rtype).Elem()
	if isJSON {
		if err := json.NewDecoder(stdout).Decode(value.Addr().Interface()); err != nil {
			return value, xray.New(err)
		}
		return value, nil
	}
	switch {
	case rtype.Kind() == reflect.String:
		value.SetString(strings.TrimSuffix(stdout.String(), "\n"))
		return value, nil
	case rtype.Kind() == reflect.Slice && rtype.Elem().Kind() == reflect.Uint8:
		value.SetBytes(append([]byte{}, stdout.Bytes()...))
		return value, nil
	case rtype.Kind() == reflect.Slice && rtype.Elem().Kind() == reflect.String:
		lines := bufio.NewReader(stdout)
		value.Set(reflect.MakeSlice(rtype, 0, 0))
		for {
			line, err := lines.ReadString('\n')
			if line != "" {
				elem := reflect.New(rtype.Elem()).Elem()
				elem.SetString(strings.TrimSuffix(line, "\n"))
				value.Set(reflect.Append(value, elem))
			}
			if err == io.EOF {
				return value, nil
			}
			if err != nil {
				return value, xray.New(err)
			}
		}
	case rtype.Kind() == reflect.Slice && !strings.Contains(stdout.String(), "\n"):
		// Any other slice has always been empty when no line of output
		// ended, there was nothing to put in it.
		value.Set(reflect.MakeSlice(rtype, 0, 0))
		return value, nil
	case rtype.Kind() == reflect.Int32:
		text := strings.TrimSuffix(stdout.String(), "\n")
		i, err := strconv.Atoi(text)
		if err != nil {
			return value, xray.New(err)
		}
		if value.OverflowInt(int64(i)) {
			return value, xray.New(&strconv.NumError{Func: "Atoi", Num: text, Err: strconv.ErrRange})
		}
		value.SetInt(int64(i))
		return value, nil
	}
	return value, fmt.Errorf("cmdl: return type %v: not implemented", rtype)
}

func zeroResults(fn api.Function) []reflect.Value {
	results := make([]reflect.Value, fn.NumOut())
	for i := range results {
		results[i] = reflect.Zero(fn.Type.Out(i))
	}
	return results
}

// outputTo discards the output when it returns a nil inherited, which is
// where [exec.Cmd] sends it.
func outputTo(capture, field, inherited io.Writer) io.Writer {
	switch {
	case capture != nil && field != nil:
		return io.MultiWriter(capture, field)
	case capture != nil:
		return capture
	case field != nil:
		return field
	}
	return inherited
}

// sourceReader remembers why a Read failed, which is the only way to tell a
// caller's reader failing from a command that stopped reading its stdin.
type sourceReader struct {
	io.Reader

	err error
}

func (source *sourceReader) Read(buf []byte) (int, error) {
	n, err := source.Reader.Read(buf)
	if err != io.EOF {
		source.err = err
	}
	return n, err
}

func needsCopying(stdin io.Reader) bool {
	_, isFile := stdin.(*os.File)
	return stdin != nil && !isFile
}

// start copies stdin itself rather than setting cmd.Stdin, because
// [exec.Cmd.Wait] cannot return until a Read of cmd.Stdin does, whatever the
// context or WaitDelay say. A Read cannot be interrupted, so the wait for this
// copy is bounded instead, and a reader that never returns strands only the
// goroutine blocked in it.
//
// What the operating system needs set up is left until here, because it
// depends on what the command is given for its stdin, stdout and stderr.
func start(cmd *exec.Cmd, stdin io.Reader) (wait func() error, err error) {
	if !needsCopying(stdin) {
		cmd.Stdin = stdin
		setupOperatingSystemSpecificsFor(cmd)
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		return cmd.Wait, nil
	}
	stdinRead, stdinWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	cmd.Stdin = stdinRead
	setupOperatingSystemSpecificsFor(cmd)
	err = cmd.Start()
	stdinRead.Close()
	if err != nil {
		stdinWrite.Close()
		return nil, err
	}
	source := &sourceReader{Reader: stdin}
	copied := make(chan struct{})
	go func() {
		defer close(copied)
		defer stdinWrite.Close()
		io.Copy(stdinWrite, source)
	}()
	return func() error {
		err := cmd.Wait()
		stdinWrite.Close()
		timeout := time.NewTimer(drainDelay)
		defer timeout.Stop()
		select {
		case <-copied:
			return cmp.Or(err, source.err)
		case <-timeout.C:
			return err
		}
	}, nil
}

type outputPipe struct {
	read, write *os.File

	exited atomic.Bool
}

// Read gives up on a pipe that stays silent after the command has exited. The
// deadline is renewed per read, not set once, because the time a consumer
// takes over what is already in the pipe must not count against it.
func (pipe *outputPipe) Read(buf []byte) (int, error) {
	if pipe.exited.Load() {
		pipe.read.SetReadDeadline(time.Now().Add(drainDelay))
	}
	return pipe.read.Read(buf)
}

func (pipe *outputPipe) commandExited() {
	pipe.exited.Store(true)
	pipe.read.SetReadDeadline(time.Now().Add(drainDelay))
}

func tee(reader io.Reader, writer io.Writer) io.Reader {
	if writer == nil {
		return reader
	}
	return io.TeeReader(reader, writer)
}

// readLines stops when send reports that nobody wants the line. A last line
// with no newline is still a line.
func readLines(reader io.Reader, send func(line []byte) bool) error {
	lines := bufio.NewReader(reader)
	for {
		line, err := lines.ReadBytes('\n')
		if len(line) > 0 && !send(bytes.TrimSuffix(line, []byte("\n"))) {
			return nil
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// streamedResults keeps the rule that has always picked out a stream: a chan
// []byte as the first result, or a chan error as the last one, which is also
// found in front of an error result.
func streamedResults(fn api.Function) (lines, errs bool) {
	if fn.NumOut() == 0 {
		return false, false
	}
	lines = fn.Type.Out(0) == reflect.TypeFor[chan []byte]()
	errs = fn.Type.Out(fn.NumOut()-1) == reflect.TypeFor[chan error]()
	return lines, errs
}

// failedCall reports err in whatever form fn's results can carry. Callers of a
// streaming function range over its channels, so they get them back closed
// instead of a panic. With neither an error result nor a chan error, stderr
// is the only place left to say why the stream is empty.
func failedCall(fn api.Function, err error) ([]reflect.Value, error) {
	lines, errs := streamedResults(fn)
	if !lines && !errs {
		return nil, err
	}
	results := zeroResults(fn)
	if lines {
		closed := make(chan []byte)
		close(closed)
		results[0] = reflect.ValueOf(closed)
	}
	if errs {
		closed := make(chan error, 1)
		if !hasErrorResult(fn) {
			closed <- err
		}
		close(closed)
		results[fn.NumOut()-1] = reflect.ValueOf(closed)
	}
	if hasErrorResult(fn) {
		return results, err
	}
	if !errs {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return results, nil
}

// unreadErrors holds what a stream has for its chan error and the caller has
// not read yet. However much that is, it is kept: a reader of stderr that
// waited for the caller would leave the command stuck writing to stderr, and
// a caller that reads the lines of stdout first waiting for a command that
// can no longer finish.
type unreadErrors struct {
	mutex    sync.Mutex
	unread   []error
	finished bool

	changed chan struct{} // holds one: there is something new to take
}

func newUnreadErrors() *unreadErrors {
	return &unreadErrors{changed: make(chan struct{}, 1)}
}

func (queue *unreadErrors) add(err error) {
	queue.mutex.Lock()
	queue.unread = append(queue.unread, err)
	queue.mutex.Unlock()
	queue.signal()
}

// finish adds how the command ended, if it failed. Nothing may be added after
// it.
func (queue *unreadErrors) finish(err error) {
	queue.mutex.Lock()
	if err != nil {
		queue.unread = append(queue.unread, err)
	}
	queue.finished = true
	queue.mutex.Unlock()
	queue.signal()
}

func (queue *unreadErrors) signal() {
	select {
	case queue.changed <- struct{}{}:
	default:
	}
}

func (queue *unreadErrors) take() (unread []error, finished bool) {
	queue.mutex.Lock()
	defer queue.mutex.Unlock()
	unread, queue.unread = queue.unread, nil
	return unread, queue.finished
}

// deliver sends what the queue is given on errs, in that order, and closes
// errs once the queue is finished. When ctx is done nobody has to be reading
// any more, so the rest is dropped, but errs still only closes after the
// command has been reaped, which is when the queue is finished.
func (queue *unreadErrors) deliver(ctx context.Context, errs chan<- error) {
	defer close(errs)
	isAbandoned := false
	for {
		unread, finished := queue.take()
		for _, err := range unread {
			if isAbandoned {
				break
			}
			select {
			case errs <- err:
			case <-ctx.Done():
				isAbandoned = true
			}
		}
		if finished {
			return
		}
		<-queue.changed
	}
}

// stream starts cmd with its output delivered on the channels among fn's
// results, and returns while it runs. A caller must read them until they
// close, or cancel ctx.
//
// The goroutine that reaps the command owns the readers: it waits for them
// before it reports how the command ended. A reader closes its pipe as it
// leaves, so a command it stopped reading fails instead of blocking. The
// reader of stderr never waits for the caller, it leaves its lines with an
// [unreadErrors], whose deliver is the only one to send on the chan error
// and the one to close it, once the command has been reaped, so nothing can
// send on a closed channel however the command ends. The chan error is
// buffered so that a caller who only reads the lines does not keep deliver
// waiting with the report of how the command exited.
func stream(ctx context.Context, fn api.Function, cmd *exec.Cmd, input cmdInput) ([]reflect.Value, error) {
	wantLines, wantErrs := streamedResults(fn)

	var err error
	var stdout, stderr outputPipe
	// What no channel receives has always been dropped, except the stderr of
	// a function with no error result, which the command inherits.
	cmd.Stdout = outputTo(nil, input.stdout, nil)
	cmd.Stderr = outputTo(nil, input.stderr, nil)
	if !hasErrorResult(fn) {
		cmd.Stderr = outputTo(nil, input.stderr, os.Stderr)
	}
	if wantLines {
		if stdout.read, stdout.write, err = os.Pipe(); err != nil {
			return failedCall(fn, xray.New(err))
		}
		cmd.Stdout = stdout.write
	}
	if wantErrs {
		if stderr.read, stderr.write, err = os.Pipe(); err != nil {
			stdout.read.Close()
			stdout.write.Close()
			return failedCall(fn, xray.New(err))
		}
		cmd.Stderr = stderr.write
	}
	wait, err := start(cmd, input.stdin)
	// Only the command may hold the write ends from here on, or the readers
	// would never see EOF.
	stdout.write.Close()
	stderr.write.Close()
	if err != nil {
		stdout.read.Close()
		stderr.read.Close()
		return failedCall(fn, xray.New(err))
	}

	var (
		results              = zeroResults(fn)
		readers              sync.WaitGroup
		stdoutErr, stderrErr error
		ended                = func(error) {}
	)
	if wantLines {
		lines := make(chan []byte)
		results[0] = reflect.ValueOf(lines)
		readers.Go(func() {
			defer close(lines)
			defer stdout.read.Close()
			stdoutErr = readLines(tee(&stdout, input.stdout), func(line []byte) bool {
				select {
				case lines <- line:
					return true
				case <-ctx.Done():
					return false
				}
			})
		})
	}
	if wantErrs {
		errs := make(chan error, 1)
		results[fn.NumOut()-1] = reflect.ValueOf(errs)
		unread := newUnreadErrors()
		ended = unread.finish
		readers.Go(func() {
			defer stderr.read.Close()
			stderrErr = readLines(tee(&stderr, input.stderr), func(line []byte) bool {
				unread.add(errors.New(string(line)))
				return ctx.Err() == nil
			})
		})
		go unread.deliver(ctx, errs)
	}
	go func() {
		err := wait()
		stdout.commandExited()
		stderr.commandExited()
		readers.Wait()
		ended(cmp.Or(err, stdoutErr, stderrErr))
	}()
	return results, nil
}
