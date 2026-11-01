package call

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"unsafe"

	"github.com/iqhive/runtime.link/api"
	"github.com/iqhive/runtime.link/api/call/internal/cgo"
	"github.com/iqhive/runtime.link/api/call/internal/cgo/dyncall"
	"github.com/iqhive/runtime.link/api/call/internal/dll"
	"github.com/iqhive/runtime.link/api/call/internal/ffi"
	"github.com/iqhive/runtime.link/api/call/internal/jit"
)

// Options
type Options struct {
	LookupSymbol func(string) (unsafe.Pointer, error)
}

// Make returns a Go function that calls the C function at jump, with the type
// described by tag, which has the format of a link tag without the symbol names.
// An error is returned if the Go function type cannot represent the tag.
func Make[T any](jump unsafe.Pointer, tag string) (T, error) {
	var fn T
	_, stype, err := ffi.ParseTag(" " + tag)
	if err != nil {
		return fn, err
	}
	compiled, err := compile(tag, jump, platform{}, reflect.TypeFor[T](), stype, nil, nil)
	if err != nil {
		return fn, err
	}
	return compiled.Interface().(T), nil
}

// API transport implements [api.Linker].
var API api.Linker[string, Options] = linker{}

type linker struct{}

func (linker) Link(structure api.Structure, lib string, opts Options) error {
	if opts.LookupSymbol != nil {
		link(opts.LookupSymbol, structure)
		return nil
	}
	var tables []dll.SymbolTable
	if lib == "" {
		lib = structure.Host.Get("lib")
	}
	for _, name := range strings.Split(lib, " ") {
		table, err := dll.Open(name)
		if err != nil {
			continue
		}
		tables = append(tables, table)
	}
	if len(tables) == 0 {
		return fmt.Errorf("library %q not available on %s", lib, runtime.GOOS)
	}
	link(inTables(tables), structure)
	return nil
}

// lookup finds the address of a named symbol.
type lookup func(string) (unsafe.Pointer, error)

func inTables(tables []dll.SymbolTable) lookup {
	return func(name string) (unsafe.Pointer, error) {
		var err error
		for _, table := range tables {
			var symbol unsafe.Pointer
			symbol, err = dll.Sym(table, name)
			if err == nil {
				return symbol, nil
			}
		}
		return nil, err
	}
}

// resolve returns the first of the given names, which are in order of
// preference, that has a symbol. A later name is not a reason to forget it.
func resolve(find lookup, names []string) (unsafe.Pointer, string, error) {
	var last error
	for _, name := range names {
		symbol, err := find(name)
		if err == nil && symbol != nil {
			return symbol, name, nil
		}
		if err != nil {
			last = err
		}
	}
	if last == nil {
		last = errors.New("no such symbol")
	}
	return nil, "", fmt.Errorf("github.com/iqhive/runtime.link/api/call unable to find %s: %w", strings.Join(names, ","), last)
}

func link(find lookup, structure api.Structure) {
	for _, fn := range structure.Functions {
		fn := fn
		tag := fn.Tags.Get("call")
		if tag == "" {
			continue
		}
		names, stype, err := ffi.ParseTag(tag)
		if err != nil {
			fn.MakeError(err)
			continue
		}
		symbol, finalName, err := resolve(find, names)
		if err != nil {
			fn.MakeError(err)
			continue
		}
		compiled, err := compile(finalName, symbol, platform{}, fn.Type, stype, fn.Args, find)
		if err != nil {
			fn.MakeError(err)
			continue
		}
		fn.Make(compiled)
	}
	for _, structure := range structure.Namespace {
		link(find, structure)
	}
}

func normal(kind reflect.Kind) reflect.Type {
	switch kind {
	case reflect.Int8:
		return reflect.TypeOf(int8(0))
	case reflect.Int16:
		return reflect.TypeOf(int16(0))
	case reflect.Int32:
		return reflect.TypeOf(int32(0))
	case reflect.Int:
		return reflect.TypeOf(int(0))
	case reflect.Int64:
		return reflect.TypeOf(int64(0))
	case reflect.Uint8:
		return reflect.TypeOf(uint8(0))
	case reflect.Uint16:
		return reflect.TypeOf(uint16(0))
	case reflect.Uint32:
		return reflect.TypeOf(uint32(0))
	case reflect.Uint:
		return reflect.TypeOf(uint(0))
	case reflect.Uint64:
		return reflect.TypeOf(uint64(0))
	case reflect.Uintptr:
		return reflect.TypeOf(uintptr(0))
	case reflect.Float32:
		return reflect.TypeOf(float32(0))
	case reflect.Float64:
		return reflect.TypeOf(float64(0))
	case reflect.Complex64:
		return reflect.TypeOf(complex64(0))
	case reflect.Complex128:
		return reflect.TypeOf(complex128(0))
	case reflect.Bool:
		return reflect.TypeOf(false)
	case reflect.String:
		return reflect.TypeOf("")
	case reflect.UnsafePointer:
		return reflect.TypeOf(unsafe.Pointer(nil))
	default:
		return nil
	}
}

// isPointer reports whether a tag's markers make the C type a pointer, as the
// name that follows them only says what is being pointed to. '-' is a macro,
// it says nothing about the type.
func isPointer(rule ffi.Type) bool {
	switch rule.Free {
	case '$', '&', '*', '~', '+':
		return true
	}
	return rule.Hash || (rule.Test.Indirect > 0 && !rule.Test.Equality.Check)
}

func isError(t reflect.Type) bool { return t == reflect.TypeFor[error]() }

// isVoid reports whether a function with the given result returns nothing,
// where 'void' with a marker is a pointer to something unknown.
func isVoid(result *ffi.Type) bool {
	return result == nil || (result.Name == "void" && !isPointer(*result))
}

func compile(name string, symbol unsafe.Pointer, abi jit.ABI, goType reflect.Type, ldType ffi.Type, goNames []string, find lookup) (reflect.Value, error) {
	plan, err := newPlan(name, abi, goType, ldType, goNames, find)
	if err != nil {
		return reflect.Value{}, err
	}
	return plan.bind(symbol)
}

// foreignFunc returns the conversion from a C function pointer into
// a Go function of type goType, with the C types taken from goType.
func foreignFunc(name string, abi jit.ABI, goType reflect.Type, find lookup) (func(unsafe.Pointer) reflect.Value, error) {
	ldType, err := foreignFuncType(goType)
	if err != nil {
		return nil, err
	}
	return foreignFuncOf(name, abi, goType, ldType, find)
}

func foreignFuncOf(name string, abi jit.ABI, goType reflect.Type, ldType ffi.Type, find lookup) (func(unsafe.Pointer) reflect.Value, error) {
	plan, err := newPlan(name, abi, goType, ldType, nil, find)
	if err != nil {
		return nil, err
	}
	return func(symbol unsafe.Pointer) reflect.Value {
		if symbol == nil {
			return reflect.Zero(goType)
		}
		fn, err := plan.bind(symbol)
		if err != nil {
			panic(err)
		}
		return fn
	}, nil
}

// plan is everything about a call that can be worked out from its types, so
// that a signature that cannot work is reported up front, not by its first call.
type plan struct {
	name   string
	abi    jit.ABI
	goType reflect.Type
	ldType ffi.Type

	send    []encoder    // each C parameter, in order.
	before  []check      // assertions on the parameters.
	kind    reflect.Type // what the C function returns, nil for void.
	after   []check      // assertions on the result.
	result  decoder      // nil unless Go wants the value of the result.
	explain *explanation // nil unless the tag names a symbol to call when 'after' fails.
}

// frame is the state of a single call.
type frame struct {
	asm    jit.Assembly
	pinner *jit.Pinner
	args   []jit.Value // as passed by Go.
	send   []jit.Value // as passed to C, for the parameters done so far.
	result jit.Value   // as returned by C.
	free   []func()    // resources that end with the call.
}

// encoder returns the value of a C parameter.
type encoder func(*frame) (jit.Value, error)

// decoder converts the value returned by C into the Go result.
type decoder func(*frame, jit.Value) jit.Value

func newPlan(name string, abi jit.ABI, goType reflect.Type, ldType ffi.Type, goNames []string, find lookup) (*plan, error) {
	p := &plan{name: name, abi: abi, goType: goType, ldType: ldType}
	if goType.Kind() != reflect.Func {
		return nil, fmt.Errorf("github.com/iqhive/runtime.link/api/call cannot link %s to the non-function type %s", name, goType)
	}
	if ldType.Name != "func" {
		return nil, fmt.Errorf("github.com/iqhive/runtime.link/api/call cannot link %s to the C type '%s', which isn't a function", name, ldType.Name)
	}
	if goType.IsVariadic() {
		return nil, fmt.Errorf("github.com/iqhive/runtime.link/api/call does not support the variadic Go function %s", goType)
	}
	if goType.NumOut() > 1 {
		return nil, fmt.Errorf("github.com/iqhive/runtime.link/api/call does not support more than one result, as in %s", goType)
	}
	if err := p.prepareArguments(goNames); err != nil {
		return nil, err
	}
	if err := p.prepareResult(find); err != nil {
		return nil, err
	}
	if err := p.prepareExplanation(find); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *plan) prepareArguments(goNames []string) error {
	var mapped int
	for _, arg := range p.ldType.Args {
		if arg.Maps > 0 {
			mapped++
		}
	}
	if mapped > p.goType.NumIn() {
		return fmt.Errorf("github.com/iqhive/runtime.link/api/call too many arguments for %s", p.name)
	}
	if mapped < p.goType.NumIn() {
		return fmt.Errorf("github.com/iqhive/runtime.link/api/call too few arguments for %s", p.name)
	}
	p.send = make([]encoder, len(p.ldType.Args))
	for i, arg := range p.ldType.Args {
		encode, err := p.encoder(i, arg, goNames)
		if err != nil {
			return err
		}
		p.send[i] = encode
	}
	for i, arg := range p.ldType.Args {
		checks, err := p.assertions(arg, p.argument(i, arg))
		if err != nil {
			return fmt.Errorf("github.com/iqhive/runtime.link/api/call argument %d of %s: %w", i+1, p.name, err)
		}
		p.before = append(p.before, checks...)
	}
	return nil
}

func (p *plan) encoder(i int, arg ffi.Type, goNames []string) (encoder, error) {
	if arg.Free == '-' {
		encode, err := p.inferred(arg)
		if err != nil {
			desc := arg.Name
			if goName := goArgName(goNames, arg, i); goName != "" {
				desc = goName + ", " + arg.Name
			}
			return nil, fmt.Errorf("github.com/iqhive/runtime.link/api/call unable to infer argument %d (%s): %w", i, desc, err)
		}
		return encode, nil
	}
	from := p.goType.In(arg.Maps - 1)
	switch {
	case arg.Name == "func":
		return funcArgument(p.name, arg, from)
	case isPointer(arg):
		return pointerArgument(arg, from)
	}
	return valueArgument(arg, from)
}

func unsupportedArgument(arg ffi.Type, from reflect.Type) error {
	return fmt.Errorf("github.com/iqhive/runtime.link/api/call does not support '%s' arguments for '%s'", from, arg.Name)
}

// funcArgument passes a Go function to C through a thunk, which is a real C
// function that calls back into Go. The address of a Go function itself is of
// no use to C: it follows Go's calling convention and expects Go's stacks.
func funcArgument(name string, arg ffi.Type, from reflect.Type) (encoder, error) {
	if isCallback(from) {
		fn := reflect.Zero(from).Interface().(callback).funcType()
		if err := matchesTag(fn, arg); err != nil {
			return nil, fmt.Errorf("github.com/iqhive/runtime.link/api/call %s: %w", name, err)
		}
		return pointerArgument(arg, from)
	}
	if from.Kind() != reflect.Func {
		return pointerArgument(arg, from)
	}
	if err := matchesTag(from, arg); err != nil {
		return nil, fmt.Errorf("github.com/iqhive/runtime.link/api/call %s: %w", name, err)
	}
	signature, err := newSignature(from)
	if err != nil {
		return nil, fmt.Errorf("github.com/iqhive/runtime.link/api/call %s: %w", name, err)
	}
	// '$' and '~' say that C keeps the pointer, so it can't be freed by the return of the call.
	retained := arg.Free == '$' || arg.Free == '~'
	return func(f *frame) (jit.Value, error) {
		var failed error
		pointer := f.asm.Foreign(f.args[arg.Maps-1], func(fn reflect.Value) unsafe.Pointer {
			if fn.IsNil() {
				return nil
			}
			thunk := dyncall.NewCallback(signature, newCallback(signature, fn))
			if thunk == nil {
				failed = errors.New("github.com/iqhive/runtime.link/api/call unable to allocate a callback")
				return nil
			}
			if !retained {
				f.free = append(f.free, thunk.Free)
			}
			return unsafe.Pointer(thunk)
		})
		return pointer, failed
	}, nil
}

// matchesTag checks a Go function that C will call against the tag that
// describes it, since C reads each argument with the type that it was told about.
func matchesTag(fn reflect.Type, tag ffi.Type) error {
	if tag.Name != "func" {
		return nil
	}
	if fn.NumIn() != len(tag.Args) {
		return fmt.Errorf("%s doesn't match a C function with %d parameters", fn, len(tag.Args))
	}
	for i, arg := range tag.Args {
		if err := matchesType(fn.In(i), arg); err != nil {
			return err
		}
	}
	if isVoid(tag.Func) != (fn.NumOut() == 0) {
		return fmt.Errorf("%s doesn't match the result of the C function", fn)
	}
	if isVoid(tag.Func) {
		return nil
	}
	return matchesType(fn.Out(0), *tag.Func)
}

func matchesType(t reflect.Type, rule ffi.Type) error {
	t = unwrapped(t)
	if rule.Name == "func" || isPointer(rule) {
		switch t.Kind() {
		case reflect.Func, reflect.Pointer, reflect.String, reflect.UnsafePointer, reflect.Uintptr:
			return nil
		}
		return fmt.Errorf("Go type %s doesn't match the C pointer '%s'", t, rule.Name)
	}
	if size := cgo.Types.LookupSize(rule.Name); size == 0 || size != t.Size() {
		return fmt.Errorf("Go type %s doesn't match the C type '%s'", t, rule.Name)
	}
	return nil
}

func pointerArgument(arg ffi.Type, from reflect.Type) (encoder, error) {
	index := arg.Maps - 1
	switch {
	case isCallback(from):
		return func(f *frame) (jit.Value, error) {
			return f.asm.Foreign(f.args[index], func(back reflect.Value) unsafe.Pointer {
				return back.Interface().(callback).pointer()
			}), nil
		}, nil
	case from.Kind() == reflect.String && arg.Name == "char":
		return func(f *frame) (jit.Value, error) {
			s := f.asm.NullTerminated(f.args[index])
			f.pinner.Pin(s)
			return s.UnsafePointer(), nil
		}, nil
	case from.Kind() == reflect.Slice && (arg.Name == "void" || arg.Name == "char"),
		from.Kind() == reflect.UnsafePointer, from.Kind() == reflect.Pointer:
		return func(f *frame) (jit.Value, error) {
			f.pinner.Pin(f.args[index])
			return f.args[index].UnsafePointer(), nil
		}, nil
	case from.Kind() == reflect.Uintptr:
		return func(f *frame) (jit.Value, error) { return f.args[index], nil }, nil
	}
	return nil, unsupportedArgument(arg, from)
}

func valueArgument(arg ffi.Type, from reflect.Type) (encoder, error) {
	index := arg.Maps - 1
	into := cgo.Types.LookupKind(arg.Name)
	target := normal(into)
	if target == nil || arg.Name == "void" {
		return nil, fmt.Errorf("github.com/iqhive/runtime.link/api/call does not support the C type '%s'", arg.Name)
	}
	switch {
	case from.Kind() == into:
		return func(f *frame) (jit.Value, error) { return f.args[index], nil }, nil
	case from.ConvertibleTo(target):
		return func(f *frame) (jit.Value, error) { return f.asm.Convert(f.args[index], target), nil }, nil
	case (from.Kind() == reflect.UnsafePointer || from.Kind() == reflect.Pointer) && from.Size() == target.Size():
		return func(f *frame) (jit.Value, error) {
			f.pinner.Pin(f.args[index])
			return f.args[index].UnsafePointer(), nil
		}, nil
	}
	return nil, unsupportedArgument(arg, from)
}

// inferred returns the encoder of a '-' flagged argument within a tag.
// for example consider the call tag `call:"read func(&char,-int=@1)"`
// the second argument can be inferred from the underlying capacity of
// the Go value passed as the first argument.
func (p *plan) inferred(rule ffi.Type) (encoder, error) {
	target := normal(cgo.Types.LookupKind(rule.Name))
	if target == nil {
		return nil, fmt.Errorf("does not support the C type '%s'", rule.Name)
	}
	literal := func(value int64) encoder {
		return func(f *frame) (jit.Value, error) { return f.asm.Convert(f.asm.Int64(value), target), nil }
	}
	eq := rule.Test.Equality
	switch {
	case !eq.Check:
		return nil, errors.New("no assertion to infer it from")
	case eq.Const != "":
		value, err := constant(eq.Const)
		if err != nil {
			return nil, err
		}
		return literal(value), nil
	case eq.Index == 0:
		return literal(eq.Value), nil
	}
	from, index, err := p.source(eq)
	if err != nil {
		return nil, err
	}
	if rule.Test.Indirect > 0 {
		size, err := elementSize(from)
		if err != nil {
			return nil, err
		}
		return literal(int64(size)), nil
	}
	switch {
	case from.Kind() == reflect.Slice:
		return func(f *frame) (jit.Value, error) {
			return f.asm.Convert(f.asm.SliceLen(f.args[index]), target), nil
		}, nil
	case from.Kind() == reflect.String:
		return func(f *frame) (jit.Value, error) {
			return f.asm.Convert(f.asm.StringLen(f.args[index]), target), nil
		}, nil
	case isInteger(from.Kind()):
		return func(f *frame) (jit.Value, error) { return f.asm.Convert(f.args[index], target), nil }, nil
	}
	return nil, fmt.Errorf("does not support inferring from '%s'", from.Kind())
}

// source returns the Go argument that '@n' refers to. n counts the
// parameters in the tag, which includes those without a Go argument.
func (p *plan) source(arg ffi.Argument) (reflect.Type, int, error) {
	if int(arg.Index) > len(p.ldType.Args) {
		return nil, 0, fmt.Errorf("@%d is out of range", arg.Index)
	}
	maps := p.ldType.Args[arg.Index-1].Maps
	if maps == 0 {
		return nil, 0, fmt.Errorf("@%d has no Go argument", arg.Index)
	}
	return p.goType.In(maps - 1), maps - 1, nil
}

// constant returns the value of an integer constant that is named by a tag.
func constant(name string) (int64, error) {
	text := ffi.Const(name)
	if text == "" {
		return 0, fmt.Errorf("unknown C constant '%s'", name)
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("C constant '%s' isn't a usable integer: %w", name, err)
	}
	return value, nil
}

func (p *plan) prepareResult(find lookup) error {
	rule := p.ldType.Func
	wanted := p.goType.NumOut() == 1
	if isVoid(rule) {
		if wanted {
			return fmt.Errorf("github.com/iqhive/runtime.link/api/call %s has no result for %s to return", p.name, p.goType)
		}
		return nil
	}
	pointer := rule.Name == "func" || isPointer(*rule)
	p.kind = reflect.TypeFor[unsafe.Pointer]()
	if !pointer {
		p.kind = normal(cgo.Types.LookupKind(rule.Name))
	}
	if p.kind == nil {
		return fmt.Errorf("github.com/iqhive/runtime.link/api/call does not support '%s' results", rule.Name)
	}
	checks, err := p.assertions(*rule, subject{from: p.kind, read: func(f *frame) jit.Value { return f.result }})
	if err != nil {
		return fmt.Errorf("github.com/iqhive/runtime.link/api/call result of %s: %w", p.name, err)
	}
	p.after = checks
	if !wanted {
		return nil
	}
	into := p.goType.Out(0)
	if !isError(into) {
		p.result, err = p.decoder(*rule, pointer, into, find)
		return err
	}
	if len(p.after) == 0 {
		failure, err := statusCheck(p.kind)
		if err != nil {
			return fmt.Errorf("github.com/iqhive/runtime.link/api/call result of %s: %w", p.name, err)
		}
		p.after = []check{failure}
	}
	return nil
}

func (p *plan) decoder(rule ffi.Type, pointer bool, into reflect.Type, find lookup) (decoder, error) {
	nonZero := func(f *frame, value jit.Value) jit.Value {
		return f.asm.Convert(f.asm.Not(f.asm.IsZero(value)), into)
	}
	convert := func(f *frame, value jit.Value) jit.Value { return f.asm.Convert(value, into) }
	unsupported := fmt.Errorf("github.com/iqhive/runtime.link/api/call does not support %s -> %s results", rule.Name, into)
	if !pointer {
		switch {
		case into.Kind() == reflect.Bool:
			return nonZero, nil
		case into.Kind() == reflect.String: // numbers convert to a string as a rune, which C didn't return.
			return nil, unsupported
		case p.kind.ConvertibleTo(into):
			return convert, nil
		}
		return nil, unsupported
	}
	switch into.Kind() {
	case reflect.Bool:
		return nonZero, nil
	case reflect.Uintptr, reflect.UnsafePointer, reflect.Pointer:
		return convert, nil
	case reflect.String:
		// the result has to be copied, which leaves nowhere to keep the pointer that the caller must free.
		if rule.Name != "char" || rule.Free == '$' {
			return nil, unsupported
		}
		return func(f *frame, value jit.Value) jit.Value {
			return f.asm.Go(value, func(pointer unsafe.Pointer) reflect.Value {
				return reflect.ValueOf(dyncall.GoString(pointer)).Convert(into)
			})
		}, nil
	case reflect.Func:
		if rule.Name != "func" {
			return nil, unsupported
		}
		function, err := foreignFuncOf(p.name, p.abi, into, rule, find)
		if err != nil {
			return nil, err
		}
		return func(f *frame, value jit.Value) jit.Value { return f.asm.Go(value, function) }, nil
	}
	return nil, unsupported
}

func (p *plan) prepareExplanation(find lookup) error {
	why := p.ldType.Call
	if why.Name == "" {
		return nil
	}
	if find == nil {
		return fmt.Errorf("github.com/iqhive/runtime.link/api/call has no symbols to look up %s in", why.Name)
	}
	symbol, _, err := resolve(find, []string{why.Name})
	if err != nil {
		return err
	}
	p.explain = &explanation{name: why.Name, symbol: symbol}
	for _, arg := range why.Args {
		switch {
		case arg.Index > 0:
			if int(arg.Index) > len(p.ldType.Args) {
				return fmt.Errorf("github.com/iqhive/runtime.link/api/call %s: @%d is out of range", why.Name, arg.Index)
			}
			index := int(arg.Index) - 1
			p.explain.args = append(p.explain.args, func(f *frame) jit.Value { return f.send[index] })
		case arg.Const != "":
			value, err := constant(arg.Const)
			if err != nil {
				return fmt.Errorf("github.com/iqhive/runtime.link/api/call %s: %w", why.Name, err)
			}
			p.explain.args = append(p.explain.args, func(f *frame) jit.Value { return f.asm.Int64(value) })
		default:
			p.explain.args = append(p.explain.args, func(f *frame) jit.Value { return f.asm.Int64(arg.Value) })
		}
	}
	return nil
}

// explanation is the symbol that a tag says to call when the result fails an assertion.
type explanation struct {
	name   string
	symbol unsafe.Pointer
	args   []func(*frame) jit.Value
}

// describe returns the error extended with what the symbol had to say about it.
func (why *explanation) describe(f *frame, abi jit.ABI, cause error) error {
	send := make([]jit.Value, len(why.args))
	for i, arg := range why.args {
		send[i] = arg(f)
	}
	info, _, err := f.asm.UnsafeCall(abi, why.symbol, send, reflect.TypeFor[int32]())
	if err != nil {
		return fmt.Errorf("%w (%s failed: %w)", cause, why.name, err)
	}
	return fmt.Errorf("%w (%s returned %v)", cause, why.name, info[0].Reflect().Int())
}

func (p *plan) bind(symbol unsafe.Pointer) (reflect.Value, error) {
	return jit.MakeFunc(p.goType, func(asm jit.Assembly, args []jit.Value) ([]jit.Value, error) {
		return p.call(symbol, asm, args)
	})
}

func (p *plan) call(symbol unsafe.Pointer, asm jit.Assembly, args []jit.Value) ([]jit.Value, error) {
	f := frame{asm: asm, pinner: asm.Pinner(), args: args, send: make([]jit.Value, len(p.send))}
	defer f.pinner.Unpin()
	defer func() {
		for _, free := range f.free {
			free()
		}
	}()
	for i, encode := range p.send {
		value, err := encode(&f)
		if err != nil {
			return nil, err
		}
		f.send[i] = value
	}
	if err := firstFailure(&f, p.name, p.before); err != nil {
		return p.fail(&f, err)
	}
	call, _, err := asm.UnsafeCall(p.abi, symbol, f.send, p.kind)
	if err != nil {
		return nil, err
	}
	if p.kind == nil {
		return nil, nil
	}
	f.result = call[0]
	failure := firstFailure(&f, p.name, p.after)
	if failure != nil && p.explain != nil {
		failure = p.explain.describe(&f, p.abi, failure)
	}
	if failure != nil {
		return p.fail(&f, failure)
	}
	switch {
	case p.goType.NumOut() == 0:
		return nil, nil
	case p.result == nil:
		return []jit.Value{asm.NewError(nil)}, nil
	}
	return []jit.Value{p.result(&f, call[0])}, nil
}

// fail hands an assertion failure to Go as the function's error, for
// a function that doesn't have one [jit.MakeFunc] will turn it into a panic.
func (p *plan) fail(f *frame, err error) ([]jit.Value, error) {
	if p.goType.NumOut() == 1 && isError(p.goType.Out(0)) {
		return []jit.Value{f.asm.NewError(err)}, nil
	}
	return nil, err
}

func goArgName(goNames []string, arg ffi.Type, i int) string {
	if arg.Maps > 0 && int(arg.Maps) <= len(goNames) {
		return goNames[arg.Maps-1]
	}
	if i >= 0 && i < len(goNames) {
		return goNames[i]
	}
	return ""
}
