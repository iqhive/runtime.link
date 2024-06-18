package source

import (
	"fmt"
	"io"
)

type StatementDecrement Star

func (stmt StatementDecrement) compile(w io.Writer, tabs int) error {
	value, _ := stmt.WithLocation.Value.Get()
	if err := value.compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, "-=1")
	return nil
}
