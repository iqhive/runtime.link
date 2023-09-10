package posix

import (
	"context"

	"runtime.link/cmd"
)

// ArchiveCommand type.
type ArchiveCommand StandardArchiveCommand[ArchiveOptions]

// StandardCatCommand line program.
type StandardArchiveCommand[Options interface{ ar() ArchiveOptions }] struct {
	cmd.Line `cmd:"ar"
		manipulates archive files.`

	Add func(ctx context.Context, archive Path, files Paths, opts *Options) error `cmd:"%[3]v -r %[1]v %[2]v"
		files (if they do not already exist) to the archive (which is created if it doesn't exist).`
	Append func(ctx context.Context, archive Path, files Paths, opts *Options) error `cmd:"%[3]v -q %[1]v %[2]v"
		files to the archive, without checking if they are already there.`
	InsertAfter func(ctx context.Context, after Path, archive Path, files Paths, opts *Options) error `cmd:"%[4]v -a %[3]v %[1]v %[2]v"
		files to the archive after the specified file path in the argument.`
	InsertBefore func(ctx context.Context, before Path, archive Path, files Paths, opts *Options) error `cmd:"%[4]v -b %[3]v %[1]v %[2]v"
		files to the archive before the specified file path in the argument.`
	Delete func(ctx context.Context, archive Path, files Paths, opts *Options) error `cmd:"%[3]v -d %[1]v %[2]v"
		files from the archive.`
	MoveAfter func(ctx context.Context, after Path, archive Path, files Paths, opts *Options) error `cmd:"%[4]v -m -a %[3]v %[1]v %[2]v"
		files to the archive after the specified file path in the argument.`
	MoveBefore func(ctx context.Context, before Path, archive Path, files Paths, opts *Options) error `cmd:"%[4]v -m -b %[3]v %[1]v %[2]v"
		files to the archive before the specified file path in the argument.`
	MoveToEnd func(ctx context.Context, archive Path, files Paths, opts *Options) error `cmd:"%[3]v -m %[1]v %[2]v"
		files to the end of the archive.`
	Print func(ctx context.Context, archive Path, opts *Options) error `cmd:"%[2]v -p %[1]v"
		the contents of the archive.`
	List func(ctx context.Context, archive Path, opts *Options) ([]string, error) `cmd:"%[2]v -t %[1]v"
		the files in the archive.`
	Extract func(ctx context.Context, archive Path, opts *Options) error `cmd:"%[2]v -x %[1]v"
		the files in the archive.`
}

// ArchiveOptions for [ArchiveCommand].
type ArchiveOptions struct {
	Common

	Verbose bool `cmd:"-v"
		output.`
	Diagonistics bool `cmd:"-c,invert"
		returns internal diagnostics as an error.`
	ProtectAgainstOverwrites bool `cmd:"-C"
		prevents overwriting files on extraction.`
	ForceEnableSymbolTable bool `cmd:"-s"
		forces the symbol table to be in-place.`
	EnableTruncation bool `cmd:"-T"
		will allow extracted files to be truncated on
		platforms that don't support the filename's length.`
	UpdateFiles bool `cmd:"-u"
		update files in the archive if the corresponding
		files on disk are newer.`
	TemporaryDirectory Path `cmd:"TMPDIR,env,omitempty"`
}

func (args ArchiveOptions) ar() ArchiveOptions { return args }
