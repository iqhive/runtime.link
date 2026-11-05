package cmdl

import (
	"bytes"
	"context"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/xray"
)

type System struct {
	Dir string // equivalent to os.Getwd()

	Args    []string  // equivalent to os.Args
	Environ []string  // equivalent to os.Environ()
	Stdin   io.Reader // equivalent to os.Stdin
	Stdout  io.Writer // equivalent to os.Stdout
	Stderr  io.Writer // equivalent to os.Stderr

	FS fs.FS // equivalent to os.DirFS()
}

func wd() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

var system = System{
	Dir:     wd(),
	Args:    os.Args,
	Environ: os.Environ(),
	Stdin:   os.Stdin,
	Stdout:  os.Stdout,
	Stderr:  os.Stderr,
	FS:      nativeFS{},
}

// Execute is the entry point for a command-line interface.
func Main(program any) {
	if err := system.Run(api.StructureOf(program)); err != nil {
		os.Stderr.WriteString(err.Error())
		os.Stderr.WriteString("\n")
		os.Exit(1)
	}
	os.Exit(0)
}

func (os System) Output(program any) ([]byte, error) {
	if os.Stdout != nil {
		return nil, errors.New("cmdl: Stdout already set")
	}
	var buf bytes.Buffer
	os.Stdout = &buf
	if err := os.Run(program); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type nativeFS struct{}

func (nativeFS) Open(name string) (fs.File, error) { return os.Open(name) }

func fitsFlag(arg, prefix, suffix string) bool {
	return len(arg) >= len(prefix)+len(suffix) && strings.HasPrefix(arg, prefix) && strings.HasSuffix(arg, suffix)
}

// isFlagField reports whether arguments may set field. An untagged field
// would otherwise match an empty argument, and one tagged "-" a lone dash.
func isFlagField(field reflect.StructField) bool {
	tag := field.Tag.Get("cmdl")
	return field.IsExported() && tag != "" && tag != "-"
}

// isFlagStruct reports whether rtype holds flags, as opposed to being a value
// that happens to be a struct, such as a [time.Time].
func isFlagStruct(rtype reflect.Type) bool {
	return hasFlags(rtype, make(map[reflect.Type]bool))
}

// hasFlags tracks what it has seen because a struct can embed a pointer to
// itself, and a stack overflow cannot be recovered from.
func hasFlags(rtype reflect.Type, seen map[reflect.Type]bool) bool {
	if rtype.Kind() != reflect.Struct || seen[rtype] {
		return false
	}
	seen[rtype] = true
	for i := 0; i < rtype.NumField(); i++ {
		field := rtype.Field(i)
		if field.Tag.Get("cmdl") != "" {
			return true
		}
		if field.Anonymous && hasFlags(pointee(field.Type), seen) {
			return true
		}
	}
	return false
}

func pointee(rtype reflect.Type) reflect.Type {
	if rtype.Kind() == reflect.Pointer {
		return rtype.Elem()
	}
	return rtype
}

func isEmbeddedFlags(field reflect.StructField) bool {
	return field.Anonymous && field.Tag.Get("cmdl") != "-" && isFlagStruct(pointee(field.Type))
}

type flagField struct {
	reflect.StructField

	value reflect.Value
}

// flagFields promotes the flags of embedded structs. A nil
// embedded pointer is filled in up front: a promoted field would panic on it,
// and no flag can say which embedded struct it needs. Only an unexported
// pointer cannot be, and its flags stay out of reach.
func flagFields(value reflect.Value, seen map[reflect.Type]bool) []flagField {
	var (
		rtype  = value.Type()
		fields []flagField
	)
	seen[rtype] = true
	for i := 0; i < rtype.NumField(); i++ {
		field := rtype.Field(i)
		if !isEmbeddedFlags(field) {
			if isFlagField(field) {
				fields = append(fields, flagField{field, value.Field(i)})
			}
			continue
		}
		if seen[pointee(field.Type)] {
			continue
		}
		embedded := value.Field(i)
		if embedded.Kind() == reflect.Pointer && embedded.IsNil() && !embedded.CanSet() {
			continue
		}
		if embedded.Kind() == reflect.Pointer && embedded.IsNil() {
			embedded.Set(reflect.New(field.Type.Elem()))
		}
		fields = append(fields, flagFields(reflect.Indirect(embedded), seen)...)
	}
	return fields
}

// isPlainPlaceholder reports whether format leaves it to the type of the
// value how its text reads, unlike a verb such as %x.
func isPlainPlaceholder(format string) bool {
	isIndexed := strings.HasPrefix(format, "%[") && strings.HasSuffix(format, "]v")
	return format == "%v" || isIndexed || strings.HasPrefix(format, "{")
}

// scanText sets value from text the way package fmt has always read it, so
// that what fmt accepts still goes, such as space around a value or 'TRUE'.
// fmt stops at the first byte it cannot use, which would read "12abc" as 12,
// so anything left over but space is an error.
func scanText(value reflect.Value, format, text string) error {
	if value.Kind() == reflect.Pointer {
		value.Set(reflect.New(value.Type().Elem()))
		return scanText(value.Elem(), format, text)
	}
	if unmarshaler, ok := value.Addr().Interface().(encoding.TextUnmarshaler); ok {
		err := unmarshaler.UnmarshalText([]byte(text))
		if at, ok := parseTimeString(text); err != nil && ok && value.Type() == reflect.TypeFor[time.Time]() {
			value.Set(reflect.ValueOf(at))
			return nil
		}
		return xray.New(err)
	}
	if isPlainPlaceholder(format) {
		format = "%v"
		if value.Kind() == reflect.String {
			value.SetString(text)
			return nil
		}
	}
	reader := strings.NewReader(text)
	if _, err := fmt.Fscanf(reader, format, value.Addr().Interface()); err != nil {
		return xray.New(err)
	}
	if rest := text[len(text)-reader.Len():]; strings.TrimSpace(rest) != "" {
		return fmt.Errorf("invalid value %q", text)
	}
	return nil
}

// parseTimeString reads what [time.Time.String] prints, which is what a link
// passes for a time.Time flag, monotonic clock reading and all. A zone with
// no name prints its offset twice, which the MST of a layout cannot read.
func parseTimeString(text string) (time.Time, bool) {
	text, _, _ = strings.Cut(text, " m=")
	for _, layout := range []string{"2006-01-02 15:04:05.999999999 -0700 MST", "2006-01-02 15:04:05.999999999 -0700 -0700"} {
		if at, err := time.Parse(layout, text); err == nil {
			return at, true
		}
	}
	return time.Time{}, false
}

func flagName(field flagField) string {
	name, _, _ := strings.Cut(field.Tag.Get("cmdl"), ",")
	return name
}

// fitsField reports whether arg is the flag of field, all of it, including
// the text after its verb.
func fitsField(field flagField, arg string) bool {
	prefix, verb, suffix := splitFlag(flagName(field))
	switch {
	case verb == "":
		return arg == prefix
	case strings.HasSuffix(prefix, " "):
		return arg == strings.TrimSuffix(prefix, " ")
	}
	return fitsFlag(arg, prefix, suffix)
}

// fitsOldField is how a string flag used to match: arg starts with what comes
// before the first '%' of its tag. A string took the rest of arg, whatever
// followed, which is all an argument that fitsField rejects can still be.
func fitsOldField(field flagField, arg string) bool {
	name := flagName(field)
	if field.Type.Kind() != reflect.String || !hasVerb(name) {
		return false
	}
	prefix, _, _ := strings.Cut(name, "%")
	return !strings.HasSuffix(prefix, " ") && strings.HasPrefix(arg, prefix)
}

// matchFlag prefers a flag that the whole of arg fits, so that the text after
// a verb tells flags apart.
func matchFlag(fields []flagField, arg string) (field flagField, isOld, found bool) {
	for _, field := range fields {
		if fitsField(field, arg) {
			return field, false, true
		}
	}
	for _, field := range fields {
		if fitsOldField(field, arg) {
			return field, true, true
		}
	}
	return flagField{}, false, false
}

func (os System) consume(value reflect.Value, tracker int) (int, error) {
	var (
		fields = flagFields(value, make(map[reflect.Type]bool))
		extra  = 0
	)
	for _, field := range fields {
		if field.Type.Kind() == reflect.Bool && strings.Contains(field.Tag.Get("cmdl"), ",invert") {
			field.value.SetBool(true)
		}
	}
	// Flags end at the first argument that is not one, which is left for
	// what comes next in the tag.
	for tracker < len(os.Args) {
		field, isOld, found := matchFlag(fields, os.Args[tracker])
		if !found {
			break
		}
		count, err := os.setFlag(field, tracker, isOld)
		if err != nil {
			return extra, err
		}
		tracker += count
		extra += count
	}
	return extra, nil
}

// setFlag sets field from the flag at tracker, and returns how many arguments
// that took.
func (os System) setFlag(field flagField, tracker int, isOld bool) (int, error) {
	var (
		arg                  = os.Args[tracker]
		name, opts, _        = strings.Cut(field.Tag.Get("cmdl"), ",")
		prefix, verb, suffix = splitFlag(name)
		text                 = arg
		taken                = 1
	)
	switch {
	case isOld:
		oldPrefix, _, _ := strings.Cut(name, "%")
		field.value.SetString(strings.TrimPrefix(arg, oldPrefix))
		return taken, nil
	case verb != "" && strings.HasSuffix(prefix, " "):
		// "-name %v" takes its value from the next argument.
		if tracker+1 >= len(os.Args) {
			return 0, fmt.Errorf("missing value for %s", arg)
		}
		taken = 2
		next := os.Args[tracker+1]
		cut, fits := strings.CutSuffix(next, suffix)
		switch {
		case fits:
			text = cut
		case field.Type.Kind() == reflect.String:
			// A string used to take the whole of it.
			text = next
		default:
			text, verb = next, oldFormat(name)
		}
	case verb != "":
		text = arg[len(prefix) : len(arg)-len(suffix)]
	}
	switch field.Type.Kind() {
	case reflect.Bool:
		val := true
		if verb != "" {
			if err := scanText(reflect.ValueOf(&val).Elem(), verb, text); err != nil {
				return 0, err
			}
		}
		if strings.Contains(opts, "invert") {
			val = !val
		}
		field.value.SetBool(val)
	case reflect.String:
		field.value.SetString(text)
	case reflect.Struct:
		// A flag with no value of its own introduces a group of flags,
		// which start after it.
		if isFlagStruct(field.Type) && verb == "" {
			count, err := os.consume(field.value, tracker+1)
			return taken + count, err
		}
		fallthrough
	default:
		if err := scanText(field.value, verb, text); err != nil {
			return 0, err
		}
	}
	return taken, nil
}

// oldFormat is what a flag's value used to be scanned with: its tag from the
// first '%' on.
func oldFormat(name string) string {
	_, format, _ := strings.Cut(name, "%")
	return "%" + format
}

func newArguments(fn api.Function) []reflect.Value {
	var args = make([]reflect.Value, 0, fn.NumIn())
	for i := 0; i < fn.NumIn(); i++ {
		args = append(args, reflect.New(fn.In(i)).Elem())
	}
	return args
}

func (os System) Run(program any) error {
	spec := api.StructureOf(program)

	fn, ok, err := os.match(spec)
	if err != nil {
		return err
	}

	if !ok && len(os.Args) > 1 {
		return fmt.Errorf("unknown command: %s", os.Args[1])
	}
	if !ok {
		fmt.Fprintln(os.Stdout, spec.Docs)
		for fn := range spec.Iter() {
			if !isOffered(commandWords(spec, fn)) {
				continue
			}
			line := strings.Join(append(namespaceWords(spec, fn), usageLine(fn)), " ")
			fmt.Fprintln(os.Stdout, strings.TrimSpace(line))
		}
		return nil
	}
	args, err := os.arguments(spec, fn)
	if err != nil {
		return err
	}
	ret, err := fn.Call(context.Background(), args)
	if err != nil {
		return err
	}
	switch len(ret) {
	case 0:
		return nil
	case 1:
		val := ret[0]
		if strings.Contains(fn.Tags.Get("cmdl"), ",json") {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "\t")
			if err := enc.Encode(val.Interface()); err != nil {
				return err
			}
			return nil
		}
		switch val.Kind() {
		case reflect.String:
			fmt.Fprintln(os.Stdout, val.String())
		case reflect.Slice:
			for i := 0; i < val.Len(); i++ {
				fmt.Fprintln(os.Stdout, val.Index(i).Interface())
			}
		default:
			fmt.Fprintln(os.Stdout, val.Interface())
		}
	}
	return nil
}

func (os System) unknownCommand() error {
	return fmt.Errorf("unknown command: %s", strings.Join(os.Args[1:], " "))
}

// arguments reads the arguments of fn from os.Args. Any left over once the
// tag is read are ignored, as they always were, and so is a value that does
// not fit around the placeholder inside its word.
func (os System) arguments(spec api.Structure, fn api.Function) ([]reflect.Value, error) {
	var (
		args       = newArguments(fn)
		scanner    = api.NewNamedArgumentScanner(args, fn.Args)
		tracker    = 1
		afterFlags = false
	)
	for _, word := range commandWords(spec, fn) {
		isPresent := tracker < len(os.Args)
		if word.isLiteral() {
			// match can only compare words whose position is fixed, so one
			// after flags may come after the unknown arguments that ended
			// them, which are passed over like any other.
			at := os.find(word, tracker)
			switch {
			case at == tracker, afterFlags && at > 0:
				tracker = at + 1
			case !isPresent && word.isOptional():
			default:
				return nil, os.unknownCommand()
			}
			continue
		}
		values, err := scanAll(&scanner, word.formats)
		switch {
		case err != nil && !isPresent:
			// Arguments that ran out before a placeholder left it unread.
			continue
		case err != nil:
			return nil, err
		case word.isWhole() && isFlagStruct(values[0].Type()):
			count, err := os.consume(values[0], tracker)
			if err != nil {
				return nil, err
			}
			tracker += count
			afterFlags = true
			continue
		case !isPresent:
			continue
		}
		isVariadic := word.isWhole() && fn.Type.IsVariadic() && values[0].Addr().Interface() == args[len(args)-1].Addr().Interface()
		if isVariadic {
			if values[0].Type().Elem() != reflect.TypeOf("") {
				return nil, fmt.Errorf("cannot set %s to %s", values[0].Type(), os.Args[tracker])
			}
			values[0].Set(reflect.ValueOf(os.Args[tracker:]))
			tracker = len(os.Args)
			continue
		}
		texts, fits := word.texts(os.Args[tracker])
		tracker++
		if !fits && strings.HasPrefix(word.written, "%") {
			return nil, os.unknownCommand()
		}
		for i, text := range texts {
			if err := os.setValue(values[i], word.formats[i], text); err != nil {
				return nil, err
			}
		}
	}
	if fn.Type.IsVariadic() {
		slice := args[len(args)-1]
		args = args[:len(args)-1]
		for i := 0; i < slice.Len(); i++ {
			args = append(args, slice.Index(i))
		}
	}
	return args, nil
}

func scanAll(scanner *api.ArgumentScanner, formats []string) ([]reflect.Value, error) {
	values := make([]reflect.Value, len(formats))
	for i, format := range formats {
		value, err := scanner.Scan(format)
		if err != nil {
			return nil, err
		}
		values[i] = value
	}
	return values, nil
}

func (os System) setValue(value reflect.Value, format, text string) error {
	switch value.Kind() {
	case reflect.Interface:
		if !reflect.TypeFor[fs.File]().Implements(value.Type()) {
			return fmt.Errorf("cannot set %s to %s", value.Type(), text)
		}
		file, err := os.FS.Open(text)
		if err != nil {
			return err
		}
		value.Set(reflect.ValueOf(file))
	case reflect.String:
		value.SetString(text)
	case reflect.Slice:
		return fmt.Errorf("cannot set %s to %s", value.Type(), text)
	default:
		return scanText(value, format, text)
	}
	return nil
}

// find returns where word is among the arguments from tracker on, or -1.
func (os System) find(word commandWord, tracker int) int {
	for i := tracker; i < len(os.Args); i++ {
		if word.fits(os.Args[i]) {
			return i
		}
	}
	return -1
}

func (os System) match(spec api.Structure) (api.Function, bool, error) {
	var match struct {
		api.Function

		Found bool
		Score int
	}
	if len(os.Args) <= 1 {
		return match.Function, false, nil
	}
	for fn := range spec.Iter() {
		score, ok := os.score(fn, commandWords(spec, fn))
		if ok && score > match.Score {
			match.Function = fn
			match.Found = true
			match.Score = score
		}
	}
	return match.Function, match.Found, nil
}

// score ranks how well the arguments fit words, and reports false if they
// cannot be that command. It counts what it always has, 2 for a word that is
// there and 1 for a placeholder, up to the first word that is not, so that
// the same command still wins. A word of the command that is missing or
// different rules it out instead, the way it never runs a sibling that shares
// a prefix, unless it comes after flags, whose words Run has to find.
func (os System) score(fn api.Function, words []commandWord) (int, bool) {
	if len(words) == 0 {
		return 0, false
	}
	var (
		scanner    = api.NewNamedArgumentScanner(newArguments(fn), fn.Args)
		score      = 0
		stopped    = false
		afterFlags = false
	)
	for i, word := range words {
		var (
			isPresent     = i+1 < len(os.Args)
			fits          = isPresent && word.fits(os.Args[i+1])
			isPlaceholder = word.isWhole() || strings.HasPrefix(word.written, "%")
		)
		switch {
		case isPlaceholder:
			// A word that starts with '%' used to be scanned for, which
			// only worked for a placeholder.
			if isPresent && !fits && !afterFlags {
				return 0, false
			}
			if !stopped {
				score++
			}
		case fits:
			if !stopped {
				score += 2
			}
		case word.isLiteral() && !afterFlags:
			return 0, false
		default:
			stopped = true
		}
		values, err := scanAll(&scanner, word.formats)
		if err == nil && word.isWhole() && isFlagStruct(values[0].Type()) {
			afterFlags = true
		}
	}
	return score, true
}

// commandWord is a word of a command, as it reads and as it was written.
type commandWord struct {
	tagWord

	written string
	also    string // the whole of a word cut at the first comma of a tag
}

func (word commandWord) fits(arg string) bool {
	if word.isLiteral() {
		return arg == word.literals[0] || (word.also != "" && arg == word.also)
	}
	_, fits := word.texts(arg)
	return fits
}

// isOptional is whether a command still runs without the word, with the zero
// value for what it holds. Only a word with no placeholder, which names the
// command, has to be there, unless it starts with '%', which used to make it
// a placeholder.
func (word commandWord) isOptional() bool {
	return !word.isLiteral() || strings.HasPrefix(word.written, "%")
}

// texts returns the text of each placeholder in arg, and false if arg is not
// this word. There is no telling where values that touch end, so such a word
// only matches itself as written, leaving its values zero.
func (word commandWord) texts(arg string) ([]string, bool) {
	switch {
	case word.isWhole():
		return []string{arg}, true
	case !word.canSplit():
		return nil, arg == word.written && !strings.HasPrefix(word.written, "%")
	}
	return word.split(arg)
}

// isOffered keeps help to what it can show: the empty word of an untagged
// function, and values that touch, can only be typed in ways nobody would.
func isOffered(words []commandWord) bool {
	for _, word := range words {
		if !word.canSplit() {
			return false
		}
	}
	return len(words) > 0 && words[0].written != ""
}

func (word tagWord) canSplit() bool {
	for i := 1; i < len(word.formats); i++ {
		if word.literals[i] == "" {
			return false
		}
	}
	return true
}

// split returns the text of each placeholder in arg, and false if arg is not
// this word. A value ends at the first match of the text after it.
func (word tagWord) split(arg string) ([]string, bool) {
	rest, fits := strings.CutPrefix(arg, word.literals[0])
	if !fits {
		return nil, false
	}
	texts := make([]string, 0, len(word.formats))
	for i := 1; i < len(word.formats); i++ {
		text, after, found := strings.Cut(rest, word.literals[i])
		if !found {
			return nil, false
		}
		texts = append(texts, text)
		rest = after
	}
	text, fits := strings.CutSuffix(rest, word.literals[len(word.formats)])
	if !fits {
		return nil, false
	}
	return append(texts, text), true
}

// namespaceWords mirrors the prefix that link gives a function, so that a
// program and a link to it agree on what a command is called.
func namespaceWords(spec api.Structure, fn api.Function) []string {
	var words []string
	for _, name := range fn.Path {
		spec = spec.Namespace[name]
		words = append(words, strings.Fields(spec.Tags.Get("cmdl"))...)
	}
	return words
}

// tagWords are the words of a function's own tag, which end at its first
// comma, as they always have. ' | json' tells a link to read JSON, it is not a
// word of the command.
func tagWords(fn api.Function) []string {
	tag, _, _ := strings.Cut(fn.Tags.Get("cmdl"), ",")
	return strings.Fields(strings.TrimSuffix(tag, " | json"))
}

// isWrittenForProgram is whether a program has always read a word as it was
// written. Any word that starts with '%' was scanned for instead.
func isWrittenForProgram(component string) bool {
	return !strings.HasPrefix(component, "%")
}

func readCommand(fn api.Function, components []string) []commandWord {
	var (
		scanner = api.NewNamedArgumentScanner(newArguments(fn), fn.Args)
		parsed  = make([]tagWord, len(components))
	)
	for i, component := range components {
		parsed[i] = parseWord(component, isWrittenForProgram(component))
	}
	parsed = keepWrittenIfUnfilled(parsed, components, scanner, isWrittenForProgram)
	words := make([]commandWord, len(parsed))
	for i := range parsed {
		words[i] = commandWord{tagWord: parsed[i], written: components[i]}
	}
	return words
}

// commandWords are the words that run fn. An untagged function, outside any
// namespace, has the empty word its empty tag always split into, and the last
// word, when a comma cut it short, still matches with what followed it, which
// is how it was matched before.
func commandWords(spec api.Structure, fn api.Function) []commandWord {
	var (
		namespace = namespaceWords(spec, fn)
		own       = tagWords(fn)
		tag       = fn.Tags.Get("cmdl")
	)
	if len(namespace) == 0 && tag == "" {
		own = []string{""}
	}
	words := readCommand(fn, append(namespace, own...))
	if last := len(own) - 1; last >= 0 && strings.Contains(tag, ",") {
		whole := strings.Fields(tag)
		if last < len(whole) && strings.HasPrefix(whole[last], own[last]+",") {
			words[len(namespace)+last].also = whole[last]
		}
	}
	return words
}

func usageLabel(fn api.Function, i int) string {
	if isFlagStruct(fn.In(i)) {
		return "[--flag ...]"
	}
	if name := fn.InName(i); name != "" {
		return "<" + name + ">"
	}
	return "<value>"
}

// usageLine resolves placeholders the way Run does, so that it reads in the
// order the arguments are typed: '%[2]v %[1]v' is not the order of the
// parameters, and a line that said so would swap them without an error. A
// failed Scan is a struct field, which there is no name to show for.
func usageLine(fn api.Function) string {
	labels := make([]reflect.Value, fn.NumIn())
	for i := range labels {
		labels[i] = reflect.ValueOf(usageLabel(fn, i))
	}
	var (
		scanner = api.NewNamedArgumentScanner(labels, fn.Args)
		words   = readCommand(fn, tagWords(fn))
		line    = make([]string, 0, len(words))
	)
	for _, word := range words {
		var text strings.Builder
		for i, format := range word.formats {
			text.WriteString(word.literals[i])
			label, err := scanner.Scan(format)
			if err != nil {
				text.WriteString("<value>")
				continue
			}
			text.WriteString(label.String())
		}
		text.WriteString(word.literals[len(word.formats)])
		line = append(line, text.String())
	}
	return strings.Join(line, " ")
}
