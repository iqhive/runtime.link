package sql

import (
	"context"
	"fmt"

	"runtime.link/api/xray"
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
	customers := Open[struct {
		Customers Map[string, Customer] `sql:"testing_customers"`
	}](db).Customers

	alice := Customer{
		Name: "Alice",
		Age:  30,
	}
	bob := Customer{
		Name: "Bob",
		Age:  40,
	}

	_, err := customers.UnsafeDelete(ctx, func(s *string, c *Customer) Query {
		return Query{Slice(0, 100)}
	})
	if err != nil {
		return xray.New(err)
	}

	if err := customers.Insert(ctx, "1234", Create, alice); err != nil {
		return xray.New(err)
	}
	if err := customers.Insert(ctx, "1234", Create, bob); err != ErrDuplicate {
		return xray.New(err)
	}
	if err := customers.Insert(ctx, "4321", Create, bob); err != nil {
		return xray.New(err)
	}

	alice.Age = 29
	if err := customers.Insert(ctx, "1234", Upsert, alice); err != nil {
		return xray.New(err)
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
		return xray.New(err)
	}
	if count != 2 {
		return fmt.Errorf("expected 2 customers, got %v", count)
	}

	query = func(name *string, cus *Customer) Query {
		return Query{
			Index(&cus.Name).Equals("Alice"),
		}
	}

	results := customers.Search(ctx, query)
	if results == nil {
		return xray.New(fmt.Errorf("expected non-nil results channel"))
	}

	var found bool
	for result := range results {
		id, cus, err := result.Get()
		if err != nil {
			return xray.New(err)
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
		return xray.New(err)
	}
	if !existed {
		return fmt.Errorf("expected to delete alice")
	}

	if err := testComposites(ctx, db); err != nil {
		return xray.New(err)
	}
	return nil
}

func testComposites(ctx context.Context, db Database) error {
	type CustomString string
	type Index struct {
		Primary   CustomString
		Secondary CustomString
	}
	type Nested struct {
		Hello [3]string
		World string
	}
	type Record struct {
		Nested Nested
		Value  int32
	}
	composites := Open[struct {
		Composites Map[Index, Record] `sql:"testing_composites"`
	}](db).Composites
	var (
		index = Index{"a", "b"}
	)
	if err := composites.Insert(ctx, index, Create, Record{Value: 1}); err != nil {
		return xray.New(err)
	}
	val, ok, err := composites.Lookup(ctx, index)
	if err != nil {
		return xray.New(err)
	}
	if !ok {
		return fmt.Errorf("expected to find record")
	}
	if val.Value != 1 {
		return fmt.Errorf("expected value 1, got %v", val.Value)
	}
	return nil
}
