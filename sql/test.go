package sql

import (
	"context"
	"fmt"
)

// Test the implementation of a [Database] against the SODIUM specification.
// This function creates new 'testing_' prefixed tables in the database. If
// the test passes, the testing records are cleaned up. If the test fails,
// the testing records are left in the database to assist with debugging.
func Test(ctx context.Context, db Database) error {
	type Customer struct {
		Name string
		Age  int
	}

	customers := Open[string, Customer](db, "testing_customers")

	alice := Customer{
		Name: "Alice",
		Age:  30,
	}
	bob := Customer{
		Name: "Bob",
		Age:  40,
	}

	if err := customers.Insert(ctx, "1234", Create, alice); err != nil {
		return err
	}
	if err := customers.Insert(ctx, "1234", Create, bob); err != ErrDuplicate {
		return err
	}
	if err := customers.Insert(ctx, "4321", Create, bob); err != nil {
		return err
	}

	alice.Age = 29
	if err := customers.Insert(ctx, "1234", Upsert, alice); err != nil {
		return err
	}

	query := func(name *string, cus *Customer) Query {
		return Query{Slice(0, 100)}
	}
	patch := func(cus *Customer) Patch {
		return Patch{
			Set(&cus.Age, 22),
		}
	}
	count, err := customers.Update(ctx, query, patch)
	if err != nil {
		return err
	}
	if count != 2 {
		return fmt.Errorf("expected 2 customers, got %v", count)
	}

	query = func(name *string, cus *Customer) Query {
		return Query{
			Index(&cus.Name).Equals("Alice"),
		}
	}
	var found bool
	for result := range customers.Search(ctx, query) {
		id, cus, err := result.Get()
		if err != nil {
			return err
		}
		if id == "1234" && cus.Name == "Alice" && cus.Age == 22 {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("expected to find alice")
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
		return err
	}
	if !existed {
		return fmt.Errorf("expected to delete alice")
	}

	return nil
}
