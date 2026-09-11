package sql_test

import (
	"context"
	"testing"

	"github.com/iqhive/runtime.link/sql"
)

func TestRAM(t *testing.T) {
	if err := sql.Test(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}
