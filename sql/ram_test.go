package sql_test

import (
	"context"
	"testing"

	"runtime.link/sql"
)

func TestRAM(t *testing.T) {
	db := sql.New()
	defer any(db).(interface{ Dump() }).Dump()
	if err := sql.Test(context.Background(), db); err != nil {
		t.Fatal(err)
	}
}
