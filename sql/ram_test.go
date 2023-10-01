package sql_test

import (
	"context"
	"testing"

	"runtime.link/sql"
)

func TestRAM(t *testing.T) {
	ctx := context.Background()

	var db = sql.New()

	type Customer struct {
		Name string
		Age  int
	}

	customers := sql.Open[string, Customer](db, "customers")

	alice := Customer{
		Name: "Alice",
		Age:  30,
	}
	bob := Customer{
		Name: "Bob",
		Age:  40,
	}

	defer db.Dump()

	if err := customers.Insert(ctx, "1234", sql.Create, alice); err != nil {
		t.Fatal(err)
	}
	if err := customers.Insert(ctx, "1234", sql.Create, bob); err != sql.ErrDuplicate {
		t.Fatal("expected duplicate error")
	}
	if err := customers.Insert(ctx, "4321", sql.Create, bob); err != nil {
		t.Fatal(err)
	}

	alice.Age = 29
	if err := customers.Insert(ctx, "1234", sql.Upsert, alice); err != nil {
		t.Fatal(err)
	}

	/*patch := func(cus *Customer) sql.Patch {
		return sql.Patch{
			sql.Set(&cus.Age).Set(22),
		}
	}
	if err := customers.UpdateFunc(ctx, nil, patch); err != nil {
		t.Fatal(err)
	}

	query := func(name *string, cus *Customer) sql.Query {
		return sql.Query{
			sql.Index(&cus.Name).Equals("Alice"),
		}
	}
	var found bool
	for result := range customers.SearchFunc(ctx, query) {
		id, cus, err := result.Get()
		if err != nil {
			t.Fatal(err)
		}
		if id == "1234" && cus.Name == "Alice" && cus.Age == 22 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to find alice")
	}

	var count int
	stats := func(name *string, cus *Customer) sql.Stats {
		return sql.Stats{
			sql.Count(&count),
		}
	}
	if err := customers.OutputFunc(ctx, nil, stats); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatal("expected 2 customers")
	}

	if err := customers.Delete(ctx, "1234"); err != nil {
		t.Fatal(err)
	}*/
}
