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
	"strconv"
	"strings"

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

// flagFields promotes the flags of embedded structs, as a link does. A nil
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

// scanText sets value from the whole of text. Scanning functions such as
// [fmt.Sscanf] stop at the first byte they cannot use, which would read
// "12abc" as 12, so anything left over is an error.
func scanText(value reflect.Value, format, text string) error {
	if value.Kind() == reflect.Pointer {
		value.Set(reflect.New(value.Type().Elem()))
		return scanText(value.Elem(), format, text)
	}
	if unmarshaler, ok := value.Addr().Interface().(encoding.TextUnmarshaler); ok {
		return xray.New(unmarshaler.UnmarshalText([]byte(text)))
	}
	if isPlainPlaceholder(format) {
		format = "%v"
		switch value.Kind() {
		case reflect.String:
			value.SetString(text)
			return nil
		case reflect.Bool:
			parsed, err := strconv.ParseBool(text)
			if err != nil {
				return xray.New(err)
			}
			value.SetBool(parsed)
			return nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			parsed, err := strconv.ParseInt(text, 0, value.Type().Bits())
			if err != nil {
				return xray.New(err)
			}
			value.SetInt(parsed)
			return nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			parsed, err := strconv.ParseUint(text, 0, value.Type().Bits())
			if err != nil {
				return xray.New(err)
			}
			value.SetUint(parsed)
			return nil
		case reflect.Float32, reflect.Float64:
			parsed, err := strconv.ParseFloat(text, value.Type().Bits())
			if err != nil {
				return xray.New(err)
			}
			value.SetFloat(parsed)
			return nil
		}
	}
	reader := strings.NewReader(text)
	if _, err := fmt.Fscanf(reader, format, value.Addr().Interface()); err != nil {
		return xray.New(err)
	}
	if reader.Len() > 0 {
		return fmt.Errorf("invalid value %q", text)
	}
	return nil
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
	for consuming := tracker < len(os.Args); consuming; {
		arg := os.Args[tracker]
		consuming = false
		for _, field := range fields {
			tag := field.Tag.Get("cmdl")
			name, opts, _ := strings.Cut(tag, ",")
			prefix, format, suffix := splitFlag(name)
			hasVerb := format != ""
			matches := !hasVerb && arg == prefix
			if hasVerb {
				switch {
				case strings.HasSuffix(prefix, " "):
					// "-name %v" takes its value from the next argument, so it
					// only matches the bare flag, and only then may it consume
					// the argument after it.
					if arg != strings.TrimSuffix(prefix, " ") {
						break
					}
					if tracker+1 >= len(os.Args) {
						return extra, fmt.Errorf("missing value for %s", arg)
					}
					value, fits := strings.CutSuffix(os.Args[tracker+1], suffix)
					if !fits {
						return extra, fmt.Errorf("invalid value %q for %s", os.Args[tracker+1], arg)
					}
					matches = true
					tracker++
					extra++
					arg = value
				case fitsFlag(arg, prefix, suffix):
					matches = true
					arg = arg[len(prefix) : len(arg)-len(suffix)]
				}
			}
			if !matches {
				continue
			}
			// consumed is set by every case that takes the current argument,
			// so that it is stepped over rather than left for a positional.
			var consumed bool
			switch field.Type.Kind() {
			case reflect.Bool:
				val := matches
				if hasVerb {
					parsed, err := strconv.ParseBool(arg)
					if err != nil {
						return extra, xray.New(err)
					}
					val = parsed
				}
				consumed = true
				if strings.Contains(opts, "invert") {
					val = !val
				}
				field.value.SetBool(val)
			case reflect.String:
				field.value.SetString(arg)
				consumed = true
			case reflect.Struct:
				// A flag with no value of its own introduces a group of flags,
				// which start after it.
				if isFlagStruct(field.Type) && !hasVerb {
					count, err := os.consume(field.value, tracker+1)
					if err != nil {
						return extra, err
					}
					tracker += count
					extra += count
					consumed = true
					break
				}
				fallthrough
			default:
				if err := scanText(field.value, format, arg); err != nil {
					return extra, err
				}
				consumed = true
			}
			if consumed {
				tracker++
				extra++
				consuming = true
				break
			}
		}
		if tracker >= len(os.Args) {
			break
		}
	}
	return extra, nil
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
			if !isRunnable(commandWords(spec, fn)) {
				continue
			}
			line := strings.Join(append(namespaceWords(spec, fn), usageLine(fn)), " ")
			fmt.Fprintln(os.Stdout, strings.TrimSpace(line))
		}
		return nil
	}
	var (
		args        = newArguments(fn)
		scanner     = api.NewNamedArgumentScanner(args, fn.Args)
		tracker int = 1
	)
	for _, component := range commandWords(spec, fn) {
		word := parseWord(component)
		if word.isLiteral() {
			// match can only compare words whose position is fixed, so this
			// one may come after flags that it has not seen.
			if tracker >= len(os.Args) || os.Args[tracker] != word.literals[0] {
				return fmt.Errorf("unknown command: %s", strings.Join(os.Args[1:], " "))
			}
			tracker++
			continue
		}
		values := make([]reflect.Value, len(word.formats))
		for i, format := range word.formats {
			if values[i], err = scanner.Scan(format); err != nil {
				return err
			}
		}
		if word.isWhole() && isFlagStruct(values[0].Type()) {
			count, err := os.consume(values[0], tracker)
			if err != nil {
				return err
			}
			tracker += count
			continue
		}
		isVariadic := word.isWhole() && fn.Type.IsVariadic() && values[0].Addr().Interface() == args[len(args)-1].Addr().Interface()
		if tracker >= len(os.Args) {
			// Absence has a value of its own in both of these, so the function
			// can tell it from an empty argument. Nothing else can.
			if isVariadic || allPointers(values) {
				continue
			}
			return fmt.Errorf("missing argument: %s", usageLine(fn))
		}
		if isVariadic {
			if values[0].Type().Elem() != reflect.TypeOf("") {
				return fmt.Errorf("cannot set %s to %s", values[0].Type(), os.Args[tracker])
			}
			values[0].Set(reflect.ValueOf(os.Args[tracker:]))
			tracker = len(os.Args)
			continue
		}
		texts, fits := word.split(os.Args[tracker])
		if !fits {
			return fmt.Errorf("unknown command: %s", strings.Join(os.Args[1:], " "))
		}
		for i, value := range values {
			var arg = texts[i]
			switch value.Kind() {
			case reflect.Interface:
				if !reflect.TypeFor[fs.File]().Implements(value.Type()) {
					return fmt.Errorf("cannot set %s to %s", value.Type(), arg)
				}
				file, err := os.FS.Open(arg)
				if err != nil {
					return err
				}
				defer file.Close()
				value.Set(reflect.ValueOf(file))
			case reflect.String:
				value.SetString(arg)
			case reflect.Slice:
				return fmt.Errorf("cannot set %s to %s", value.Type(), arg)
			default:
				if err := scanText(value, word.formats[i], arg); err != nil {
					return err
				}
			}
		}
		tracker++
	}
	// An argument nothing took is a flag or subcommand this program does not
	// have. Running without it could do something the caller asked not to.
	if tracker < len(os.Args) {
		return fmt.Errorf("unexpected argument: %s", os.Args[tracker])
	}
	if fn.Type.IsVariadic() {
		slice := args[len(args)-1]
		args = args[:len(args)-1]
		for i := 0; i < slice.Len(); i++ {
			args = append(args, slice.Index(i))
		}
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
		if _, isJSON := cutJSONOption(fn.Tags.Get("cmdl")); isJSON {
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
			if val.Type().Elem().Kind() == reflect.Uint8 {
				_, err := os.Stdout.Write(val.Bytes())
				return err
			}
			for i := 0; i < val.Len(); i++ {
				fmt.Fprintln(os.Stdout, val.Index(i).Interface())
			}
		default:
			fmt.Fprintln(os.Stdout, val.Interface())
		}
	}
	return nil
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
		if ok && (!match.Found || score > match.Score) {
			match.Function = fn
			match.Found = true
			match.Score = score
		}
	}
	return match.Function, match.Found, nil
}

// score ranks how well the arguments fit words, and reports false if they
// cannot be that command. A word of the command that is missing or different
// rules it out, the way it never runs a sibling that shares a prefix.
func (os System) score(fn api.Function, words []string) (int, bool) {
	if !isRunnable(words) {
		return 0, false
	}
	var (
		scanner = api.NewNamedArgumentScanner(newArguments(fn), fn.Args)
		score   = 0
	)
	for i, component := range words {
		word := parseWord(component)
		isPresent := i+1 < len(os.Args)
		switch {
		case word.isLiteral() && isPresent && os.Args[i+1] == word.literals[0]:
			score += 2
			continue
		case word.isLiteral():
			return 0, false
		}
		for _, format := range word.formats {
			value, err := scanner.Scan(format)
			if err != nil || (word.isWhole() && isFlagStruct(value.Type())) {
				// Flags take any number of arguments, which leaves the position
				// of the words after them unknown until Run has parsed them.
				return score + 1, true
			}
		}
		// An argument short of what the command needs counts against it, so
		// that "run a" prefers "run %v" to "run %v %v". It stays in the running
		// so that Run can say what is missing.
		if !isPresent {
			score--
			continue
		}
		if word.isWhole() {
			score++
			continue
		}
		if _, fits := word.split(os.Args[i+1]); !fits {
			return 0, false
		}
		score += 2
	}
	return score, true
}

// isRunnable keeps match and help in step: no words names no command, and
// nothing can show where a value ends if the next placeholder touches it.
func isRunnable(words []string) bool {
	for _, word := range words {
		if !parseWord(word).canSplit() {
			return false
		}
	}
	return len(words) > 0
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

func allPointers(values []reflect.Value) bool {
	for _, value := range values {
		if value.Kind() != reflect.Pointer {
			return false
		}
	}
	return true
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

func tagWords(fn api.Function) []string {
	tag, _ := cutJSONOption(fn.Tags.Get("cmdl"))
	return strings.Fields(strings.TrimSuffix(tag, " | json"))
}

func commandWords(spec api.Structure, fn api.Function) []string {
	return append(namespaceWords(spec, fn), tagWords(fn)...)
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
		words   = tagWords(fn)
		line    = make([]string, 0, len(words))
	)
	for _, component := range words {
		word := parseWord(component)
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
