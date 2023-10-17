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

	defer any(db).(interface{ Dump() }).Dump()

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

	query := func(name *string, cus *Customer) sql.Query {
		return sql.Query{sql.Slice(0, 100)}
	}
	patch := func(cus *Customer) sql.Patch {
		return sql.Patch{
			sql.Set(&cus.Age, 22),
		}
	}
	count, err := customers.Update(ctx, query, patch)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatal("expected 2 customers", count)
	}

	query = func(name *string, cus *Customer) sql.Query {
		return sql.Query{
			sql.Index(&cus.Name).Equals("Alice"),
		}
	}
	var found bool
	for result := range customers.Search(ctx, query) {
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

	/*var counter atomic.Int32
	stats := func(name *string, cus *Customer) sql.Stats {
		return sql.Stats{
			sql.Count(&counter),
		}
	}
	if err := customers.Output(ctx, nil, stats); err != nil {
		t.Fatal(err)
	}
	if counter.Load() != 2 {
		t.Fatal("expected 2 customers")
	}*/

	existed, err := customers.Delete(ctx, "1234", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !existed {
		t.Fatal("expected to delete alice")
	}
}
