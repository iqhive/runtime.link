package source

import (
	"fmt"
	"io"
)

type StatementIncrement Star

func (stmt StatementIncrement) compile(w io.Writer, tabs int) error {
	value, _ := stmt.WithLocation.Value.Get()
	if err := value.compile(w, tabs); err != nil {
		return err
	}
	fmt.Fprintf(w, "+=1")
	return nil
}
