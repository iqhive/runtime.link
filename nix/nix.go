// Package nix provides a system interface derived from *NIX system standards.
package nix

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"runtime.link/log"
	"runtime.link/ram"
	"runtime.link/utc"
	"runtime.link/utc/nano"
)

// Path is a slash-separated path. Directories always end in a slash.
type Path string

// Null device, used to deleting files.
const Null Path = "/dev/null"

// File entry.
type File interface {
	Name() string
	Path() Path

	Size() ram.Bytes

	CreatedAt() utc.Time
	UpdatedAt() utc.Time

	Reader() ram.Reader
	Writer() ram.Writer
}

// Standard system.
type Standard struct {
	_ struct{}

	Path ram.Global[Path] // working directory

	Time func() utc.Time                           // current time
	Wait func(context.Context, nano.Seconds) error // sleep

	Vars map[string]string // environment variables
	Args []string          // command line arguments
	Data ram.Reader        // standard input

	Rand ram.Reader // secure random number generator
	Logs log.Writer // standard output and standard error

	Move func(ctx context.Context, from Path, into Path) error // rename file
	Pipe func(context.Context) (ram.Reader, ram.Writer, error) // pipe
	Open func(context.Context, Path) (File, error)             // stat
}

// New returns the native system implementation of the [Standard].
func New() Standard {
	vars := make(map[string]string, len(os.Environ()))
	for _, env := range os.Environ() {
		key, val, _ := strings.Cut(env, "=")
		vars[key] = val
	}
	return Standard{
		Path: wd{},
		Time: func() utc.Time { return utc.Time(time.Now()) },
		Wait: func(ctx context.Context, nanos nano.Seconds) error {
			ticker := time.NewTimer(time.Duration(nanos))
			select {
			case <-ctx.Done():
				ticker.Stop()
				return ctx.Err()
			case <-ticker.C:
				return nil
			}
		},
		Vars: vars,
		Args: os.Args,
		Data: toReader{os.Stdin},
		Rand: toReader{rand.Reader},
		Logs: log.New(logFormat{}),
		Move: func(ctx context.Context, from Path, into Path) error {
			return os.Rename(string(from), string(into))
		},
		Pipe: func(context.Context) (ram.Reader, ram.Writer, error) {
			r, w, err := os.Pipe()
			return toReader{r}, toWriter{w}, err
		},
		Open: func(ctx context.Context, path Path) (File, error) {
			info, err := os.Stat(string(path))
			if err != nil {
				return nil, err
			}
			return toFile{
				info: info,
				path: path,
			}, nil
		},
	}
}

type toFile struct {
	info os.FileInfo
	path Path
}

func (f toFile) Name() string    { return f.info.Name() }
func (f toFile) Path() Path      { return f.path }
func (f toFile) Size() ram.Bytes { return ram.Bytes(f.info.Size()) }
func (f toFile) CreatedAt() utc.Time {
	return utc.Time{}
}
func (f toFile) UpdatedAt() utc.Time {
	return utc.Time(f.info.ModTime())
}
func (f toFile) Reader() ram.Reader {
	file, err := os.Open(string(f.path))
	if err != nil {
		return nil
	}
	return toReader{file}
}
func (f toFile) Writer() ram.Writer {
	file, err := os.OpenFile(string(f.path), os.O_WRONLY, 0755)
	if err != nil {
		return nil
	}
	return toWriter{file}
}

type toReader struct {
	io.Reader
}

func (r toReader) Read(ctx context.Context, p []byte) (ram.Bytes, error) {
	n, err := r.Reader.Read(p)
	return ram.Bytes(n), err
}

type toWriter struct {
	io.Writer
}

func (w toWriter) Write(ctx context.Context, p []byte) (ram.Bytes, error) {
	n, err := w.Writer.Write(p)
	return ram.Bytes(n), err
}

type logFormat struct{}

func (logFormat) Report(ctx context.Context, err error) { fmt.Fprint(os.Stderr, err) }
func (logFormat) Record(ctx context.Context, subject any, event ...string) {
	fmt.Fprintln(os.Stdout, subject, event)
}

type wd struct{}

func (wd) Get(ctx context.Context) (Path, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return Path(path.Join(filepath.Split(dir))), err
}

func (wd) Set(ctx context.Context, dir Path) error {
	return os.Chdir(filepath.Join(strings.Split(string(dir), "/")...))
}