package cmdl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"

	"runtime.link/api"
)

type System struct {
	Dir string // equivalent to os.Getwd()

	Args    []string  // equivalent to os.Args
	Environ []string  // equivalent to os.Environ()
	Stdin   io.Reader // equivalent to os.Stdin
	Stdout  io.Writer // equivalent to os.Stdout
	Stderr  io.Writer // equivalent to os.Stderr
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

func (os System) consume(value reflect.Value, tracker int) (int, error) {
	rtype := value.Type()
	extra := 0
	for consuming := true; consuming; {
		arg := os.Args[tracker]
		consuming = false
		for i := 0; i < value.NumField(); i++ {
			field := rtype.Field(i)
			switch field.Type.Kind() {
			case reflect.Bool:
				tag := field.Tag.Get("cmdl")
				name, opts, _ := strings.Cut(tag, ",")
				val := arg == name
				if strings.Contains(name, "%") {
					prefix, format, _ := strings.Cut(name, "%")
					if strings.HasPrefix(arg, prefix) {
						val = true
					}
					if strings.HasSuffix(prefix, "=") {
						arg = strings.TrimPrefix(arg, prefix)
					} else if strings.HasSuffix(prefix, " ") {
						tracker++
						extra++
						if tracker >= len(os.Args) {
							return tracker, fmt.Errorf("missing value for %s", arg)
						}
						arg = os.Args[tracker]
						consuming = true
					}
					_, err := fmt.Sscanf(arg, "%"+format, &val)
					if err != nil {
						return tracker, err
					}
				}
				if val {
					tracker++
					extra++
					consuming = true
				}
				if strings.Contains(opts, "invert") {
					val = !val
				}
				value.Field(i).SetBool(val)
			default:
				return tracker, fmt.Errorf("unsupported type: %s", field.Type)
			}
		}

		if tracker >= len(os.Args) {
			break
		}
	}
	return extra, nil
}

func (os System) Run(program any) error {
	spec := api.StructureOf(program)

	fn, ok, err := os.match(spec)
	if err != nil {
		return err
	}

	if !ok && len(os.Args) == 1 {
		fmt.Fprintf(os.Stdout, spec.Docs)
		return nil
	} else if !ok {
		return fmt.Errorf("unknown command: " + os.Args[1])
	}
	var args = make([]reflect.Value, 0, fn.NumIn())
	for i := 0; i < fn.NumIn(); i++ {
		args = append(args, reflect.New(fn.In(i)).Elem())
	}
	var (
		scanner     = api.NewArgumentScanner(args)
		tracker int = 1
	)
	for _, component := range strings.Split(strings.Split(string(fn.Tags.Get("cmdl")), ",")[0], " ") {
		if len(component) > 0 && component[0] == '%' {
			value, err := scanner.Scan(component)
			if err != nil {
				return err
			}
			var arg = os.Args[tracker]
			switch value.Kind() {
			case reflect.Interface:
				if value.Type().Implements(reflect.TypeOf([0]context.Context{}).Elem()) {
					value.Set(reflect.ValueOf(context.Background()))
				} else {
					return fmt.Errorf("cannot set %s to %s", value.Type(), arg)
				}
			case reflect.String:
				value.SetString(arg)
			case reflect.Int64:
				if i, err := strconv.ParseInt(arg, 10, 64); err != nil {
					return fmt.Errorf("cannot set %s to %s", value.Type(), arg)
				} else {
					value.SetInt(i)
				}
			case reflect.Slice:
				switch value.Type().Elem() {
				case reflect.TypeOf(""):
					if fn.Type.IsVariadic() && value.Addr().Interface() == args[len(args)-1].Addr().Interface() {
						value.Set(reflect.ValueOf(os.Args[tracker:]))
					} else {
						return fmt.Errorf("cannot set %s to %s", value.Type(), arg)
					}
				default:
					return fmt.Errorf("cannot set %s to %s", value.Type(), arg)
				}
			case reflect.Struct:
				count, err := os.consume(value, tracker)
				if err != nil {
					return err
				}
				tracker += count
				if len(os.Args) <= tracker {
					break
				}
				continue
			default:
				return fmt.Errorf("cannot set %s to %s", value.Type(), arg)
			}
		}
		tracker++
		if len(os.Args) <= tracker {
			break
		}
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

func (os System) match(spec api.Structure) (api.Function, bool, error) {
	var match struct {
		api.Function

		Score int
	}
	if len(os.Args) == 1 {
		return match.Function, false, nil
	}
	for _, fn := range spec.Functions {
		tag := fn.Tags.Get("cmdl")

		var matching bool = true
		args := strings.Split(string(tag), " ")
		var score = 0
		for i, arg := range args {
			if len(arg) > 0 && arg[0] == '%' {
				score++
				continue
			}
			if len(os.Args) <= i+1 {
				matching = false
				break
			}
			if arg != os.Args[i+1] {
				matching = false
				break
			}
			if arg == os.Args[i+1] {
				score += 2
				continue
			}
		}
		if matching && score > match.Score {
			match.Function = fn
			match.Score = score
		}
	}
	if match.Score == 0 {
		return match.Function, false, nil
	}
	return match.Function, true, nil
}
