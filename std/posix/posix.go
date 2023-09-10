package posix

// Path is a path to a file.
type Path string

// Paths to pass to a [Command].
type Paths []Path

type Common struct {
	Language            string `cmd:"LANG,env,omitempty"`
	Locale              string `cmd:"LC_ALL,env,omitempty"`
	LocaleCharacterType string `cmd:"LC_CTYPE,env,omitempty"`
	LocaleError         string `cmd:"LC_MESSAGES,env,omitempty"`
}
