package cmdl

import (
	"bufio"
	"bytes"
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

	"runtime.link/api"
	"runtime.link/api/xray"
)

type cmdInput struct {
	args []string
	env  []string
	wd   string
}

func (input *cmdInput) add(val reflect.Value) error {
	switch val.Kind() {
	case reflect.Struct:
		rtype := val.Type()
		if rtype.Implements(reflect.TypeFor[encoding.TextMarshaler]()) {
			data, err := val.Interface().(encoding.TextMarshaler).MarshalText()
			if err != nil {
				return xray.New(err)
			}
			input.args = append(input.args, string(data))
			return nil
		}
		for i := 0; i < rtype.NumField(); i++ {
			field := rtype.Field(i)
			if !field.IsExported() {
				continue
			}
			if field.Anonymous && field.Type.Kind() == reflect.Struct {
				if err := input.add(val.Field(i)); err != nil {
					return xray.New(err)
				}
				continue
			}
			exec := field.Tag.Get("cmdl")
			if exec == "-" {
				continue
			}

			omitBooleanFlag := (!strings.Contains(exec, "%v") && field.Type.Kind() == reflect.Bool)

			if (strings.Contains(exec, ",omitempty") || strings.Contains(exec, ",omitzero") || omitBooleanFlag) && val.Field(i).IsZero() {
				continue
			}
			if strings.Contains(exec, ",env") {
				name, _, _ := strings.Cut(exec, ",")
				input.env = append(input.env, fmt.Sprintf("%s=%v", name, val.Field(i).Interface()))
				continue
			}
			if strings.Contains(exec, ",dir") {
				input.wd = fmt.Sprint(val.Field(i).Interface())
				continue
			}

			exec, _, _ = strings.Cut(exec, ",")

			parts := strings.Split(exec, " ")
			for _, part := range parts {
				if strings.HasPrefix(part, "%") || strings.Contains(part, "%") {
					input.args = append(input.args, fmt.Sprintf(part, val.Field(i).Interface()))
				} else {
					input.args = append(input.args, part)
				}
			}
		}
	case reflect.Slice:
		for i := 0; i < val.Len(); i++ {
			if err := input.add(val.Index(i)); err != nil {
				return xray.New(err)
			}
		}
	case reflect.Pointer:
		if !val.IsNil() {
			if err := input.add(val.Elem()); err != nil {
				return xray.New(err)
			}
		}
	default:
		input.args = append(input.args, fmt.Sprint(val.Interface()))
	}
	return nil
}

func linkStructure(spec api.Structure, cmd string) {
	if _, err := exec.LookPath(cmd); err != nil {
		spec.MakeError(fmt.Errorf("cannot find program '%s': %w", cmd, err))
		return
	}
	for _, fn := range spec.Functions {
		link(cmd, fn)
	}
	for _, section := range spec.Namespace {
		section.Host = spec.Host
		linkStructure(section, cmd)
	}
}

func link(cmd string, fn api.Function) {
	tag := string(fn.Tags.Get("cmdl"))
	if cmd == "" {
		cmd, tag, _ = strings.Cut(tag, " ")
	}
	var isJSON bool = false
	if strings.HasSuffix(tag, " | json") {
		tag = strings.TrimSuffix(tag, " | json")
		isJSON = true
	}
	fn.Make(func(ctx context.Context, args []reflect.Value) (results []reflect.Value, err error) {
		scanner := api.NewArgumentScanner(args)

		var execArgs cmdInput
		if tag != "" {
			for _, component := range strings.Split(string(tag), " ") {
				if strings.HasPrefix(component, "%") || strings.HasPrefix(component, "{") {
					component = strings.Trim(component, "{}")
					val, err := scanner.Scan(component)
					if err != nil {
						return nil, xray.New(err)
					}
					if err := execArgs.add(val); err != nil {
						return nil, xray.New(err)
					}
				} else {
					execArgs.args = append(execArgs.args, component)
				}
			}
		}

		var stdout bytes.Buffer
		var stderr bytes.Buffer

		stdoutRead, stdoutWrite, err := os.Pipe()
		if err != nil {
			return nil, xray.New(err)
		}
		stderrRead, stderrWrite, err := os.Pipe()
		if err != nil {
			return nil, xray.New(err)
		}
		if os.Getenv("DEBUG_CMD") != "" {
			fmt.Println(cmd, execArgs)
		}
		cmd := exec.CommandContext(ctx, cmd, execArgs.args...)
		cmd.Env = append(os.Environ(), execArgs.env...)
		cmd.Dir = execArgs.wd
		setupOperatingSystemSpecificsFor(cmd, stdoutWrite, stderrWrite)

		results = make([]reflect.Value, fn.NumOut())
		for i := range results {
			results[i] = reflect.Zero(fn.Type.Out(i))
		}

		var async bool
		var chout chan []byte
		var cherr chan error

		if fn.NumOut() > 0 && fn.Type.Out(0) == reflect.TypeOf([0]chan []byte{}).Elem() {
			cmd.Stdout = stdoutWrite
			async = true
			chout = make(chan []byte)
			results[0] = reflect.ValueOf(chout)
			go func() {
				reader := bufio.NewReader(stdoutRead)
				for {
					line, err := reader.ReadBytes('\n')
					if err != nil {
						if err != io.EOF {

						}
						close(chout)
						return
					}
					select {
					case chout <- line[:len(line)-1]:
					case <-ctx.Done():
						return
					}
				}
			}()
		} else if fn.NumOut() > 0 {
			cmd.Stdout = stdoutWrite
			go io.Copy(&stdout, stdoutRead)
		} else {
			cmd.Stdout = os.Stdout
		}

		if fn.Type.NumOut() > 0 && fn.Type.Out(fn.Type.NumOut()-1) == reflect.TypeOf([0]chan error{}).Elem() {
			cmd.Stderr = stderrWrite
			async = true
			cherr = make(chan error)
			results[1] = reflect.ValueOf(cherr)
			go func() {
				reader := bufio.NewReader(stderrRead)
				for {
					line, err := reader.ReadBytes('\n')
					if err != nil {
						if err != io.EOF {

						}
						close(cherr)
						return
					}
					select {
					case cherr <- errors.New(string(line[:len(line)-1])):
					case <-ctx.Done():
						return
					}
				}
			}()
		} else if fn.NumOut() != fn.Type.NumOut() {
			cmd.Stderr = stderrWrite
			go io.Copy(&stderr, stderrRead)
		} else {
			cmd.Stderr = os.Stderr
		}
		if async {
			if err := cmd.Start(); err != nil {
				return results, xray.New(err)
			}
			go func() {
				if err := cmd.Wait(); err != nil {
					select {
					case cherr <- err:
					case <-ctx.Done():
					}
				}
				stderrWrite.Close()
				stdoutWrite.Close()
			}()
			return
		}
		if err := cmd.Run(); err != nil {
			if text := stderr.String(); strings.TrimSpace(text) != "" {
				return nil, errors.New(text)
			}
			return nil, xray.New(err)
		}
		if fn.NumOut() > 0 {
			if isJSON {
				var result = reflect.New(fn.Type.Out(0)).Interface()
				if err := json.NewDecoder(&stdout).Decode(result); err != nil {
					return nil, xray.New(err)
				}
				return []reflect.Value{reflect.ValueOf(result).Elem()}, nil
			} else {
				if fn.NumOut() == 1 {
					var value = reflect.New(fn.Type.Out(0)).Elem()
					switch fn.Type.Out(0).Kind() {
					case reflect.String:
						result := stdout.String()
						result = strings.TrimSuffix(result, "\n")
						results[0] = reflect.ValueOf(result)
						return results, nil
					case reflect.Slice:
						var lines = bufio.NewReader(&stdout)
						var result = reflect.MakeSlice(fn.Type.Out(0), 0, 0)
						for {
							line, err := lines.ReadString('\n')
							if err != nil {
								if err == io.EOF {
									break
								}
								return nil, xray.New(err)
							}
							line = strings.TrimSuffix(line, "\n")
							elem := reflect.New(fn.Type.Out(0).Elem()).Elem()
							elem.SetString(line)
							result = reflect.Append(result, elem)
						}
						return []reflect.Value{result}, nil
					case reflect.Int32:
						result := stdout.String()
						result = strings.TrimSuffix(result, "\n")
						i, err := strconv.Atoi(result)
						if err != nil {
							return nil, xray.New(err)
						}
						value.SetInt(int64(i))
						results[0] = value
						return results, nil
					}
				}
				return nil, fmt.Errorf("cmdl: return type %v: not implemented", fn.Type.Out(0))
			}
		}
		return
	})
}
