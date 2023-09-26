package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"runtime.link/qnq"
)

func init() {
	qnq.RegisterHost(func(structure qnq.Structure) {
		if len(os.Args) > 1 {
			host(structure)
			os.Exit(0)
		}
	})
}

// Main is the entry point for a command-line interface.
func Main(program any) {
	host(qnq.StructureOf(program))
}

func host(spec qnq.Structure) {
	fn, ok, err := match(spec)
	if err != nil {
		os.Stderr.WriteString(err.Error())
		os.Stderr.WriteString("\n")
		os.Exit(1)
	}
	if !ok && len(os.Args) == 1 {
		fmt.Println(spec.Docs)
		os.Exit(0)
	} else if !ok {
		fmt.Println("unknown command: " + os.Args[1])
		os.Exit(1)
	}
	var args = make([]reflect.Value, 0, fn.Type.NumIn())
	for i := 0; i < fn.Type.NumIn(); i++ {
		args = append(args, reflect.New(fn.Type.In(i)).Elem())
	}
	var (
		scanner     = qnq.NewArgumentScanner(args)
		tracker int = 1
	)
	for _, component := range strings.Split(strings.Split(string(fn.Tags.Get("cmd")), ",")[0], " ") {
		if len(component) > 0 && component[0] == '%' {
			value, err := scanner.Scan(component)
			if err != nil {
				os.Stderr.WriteString(err.Error())
				os.Stderr.WriteString("\n")
				os.Exit(1)
			}
			var arg = os.Args[tracker]
			switch value.Kind() {
			case reflect.Interface:
				if value.Type().Implements(reflect.TypeOf([0]context.Context{}).Elem()) {
					value.Set(reflect.ValueOf(context.Background()))
				} else {
					panic(fmt.Errorf("cannot set %s to %s", value.Type(), arg))
				}
			case reflect.String:
				value.SetString(arg)
			case reflect.Int64:
				if i, err := strconv.ParseInt(arg, 10, 64); err != nil {
					panic(fmt.Errorf("cannot set %s to %s", value.Type(), arg))
				} else {
					value.SetInt(i)
				}
			case reflect.Slice:
				switch value.Type().Elem() {
				case reflect.TypeOf(""):
					if fn.Type.IsVariadic() && value.Addr().Interface() == args[len(args)-1].Addr().Interface() {
						value.Set(reflect.ValueOf(os.Args[tracker:]))
					} else {
						panic(fmt.Errorf("cannot set %s to %s", value.Type(), arg))
					}
				default:
					panic(fmt.Errorf("cannot set %s to %s", value.Type(), arg))
				}
			default:
				panic(fmt.Errorf("cannot set %s to %s", value.Type(), arg))
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
		os.Stderr.WriteString(err.Error())
		os.Stderr.WriteString("\n")
		os.Exit(1)
	}
	switch len(ret) {
	case 0:
		return
	case 1:
		val := ret[0]
		if strings.Contains(fn.Tags.Get("cmd"), ",json") {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "\t")
			if err := enc.Encode(val.Interface()); err != nil {
				os.Stderr.WriteString(err.Error())
				os.Stderr.WriteString("\n")
				os.Exit(1)
			}
			return
		}
		switch val.Kind() {
		case reflect.String:
			fmt.Println(val.String())
		case reflect.Slice:
			for i := 0; i < val.Len(); i++ {
				fmt.Println(val.Index(i).Interface())
			}
		default:
			fmt.Println(val.Interface())
		}
	}
}

func match(spec qnq.Structure) (qnq.Function, bool, error) {
	var match struct {
		qnq.Function

		Len int
	}
	if len(os.Args) == 1 {
		return match.Function, false, nil
	}
	for _, fn := range spec.Functions {
		tag := fn.Tags.Get("cmd")

		var matching bool = true
		args := strings.Split(string(tag), " ")
		for i, arg := range args {
			if len(arg) > 0 && arg[0] == '%' {
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
		}
		if matching && len(args) > match.Len {
			match.Function = fn
			match.Len = len(args)
		}
	}
	if match.Len == 0 {
		return match.Function, false, nil
	}
	return match.Function, true, nil
}
