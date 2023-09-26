package posix

import (
	"bytes"
	"io"
)

// Path is a path to a file.
type Path string

// Paths to pass to a [Command].
type Paths []Path

func (p Paths) MarshalText() ([]byte, error) {
	var buf bytes.Buffer
	for i, path := range p {
		if i > 0 {
			buf.WriteByte(':')
		}
		buf.WriteString(string(path))
	}
	return buf.Bytes(), nil
}

func (p *Paths) UnmarshalText(text []byte) error {
	var paths []Path
	for _, path := range bytes.Split(text, []byte{':'}) {
		paths = append(paths, Path(path))
	}
	*p = paths
	return nil
}

type Common struct {
	Language            string `cmd:"LANG,env,omitempty"`
	Locale              string `cmd:"LC_ALL,env,omitempty"`
	LocaleCharacterType string `cmd:"LC_CTYPE,env,omitempty"`
	LocaleError         string `cmd:"LC_MESSAGES,env,omitempty"`

	MessageCatalogSearchPaths Paths `cmd:"NLSPATH,env,omitempty"`

	Path Path `cmd:",dir,omitempty"
		changes the working directory to the given directory.`
	Reader io.Reader `cmd:",omitempty"
		reads input from the given reader instead of qnqin.`
	Writer io.Writer `cmd:",omitempty"
		writes output to the given writer instead of qnqout.`
}
