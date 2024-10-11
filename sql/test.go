package sql

import (
	"context"
	"fmt"
	"sync/atomic"

	"runtime.link/api/xray"
	"runtime.link/xyz"
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
	DB := Open[struct {
		Customers Map[string, Customer] `sql:"testing_customers"`
	}](db)

	alice := Customer{
		Name: "Alice",
		Age:  30,
	}
	bob := Customer{
		Name: "Bob",
		Age:  40,
	}

	_, err := DB.Customers.UnsafeDelete(ctx, func(s *string, c *Customer) Query {
		return Query{Slice(0, 100)}
	})
	if err != nil {
		return xray.New(err)
	}

	if err := DB.Customers.Insert(ctx, "1234", Create, alice); err != nil {
		return xray.New(err)
	}
	if err := DB.Customers.Insert(ctx, "1234", Create, bob); err != ErrDuplicate {
		return xray.New(err)
	}
	if err := DB.Customers.Insert(ctx, "4321", Create, bob); err != nil {
		return xray.New(err)
	}

	query := func(name *string, cus *Customer) Query {
		return Query{
			Order(&cus.Age).Decreasing(),
		}
	}
	found := false
	for id, cus := range DB.Customers.Search(ctx, query, &err) {
		if id == "4321" && cus.Name == "Bob" && cus.Age == 40 {
			found = true
		}
		break
	}
	if err != nil {
		return xray.New(err)
	}
	if !found {
		return fmt.Errorf("expected to find bob")
	}

	alice.Age = 29
	if err := DB.Customers.Insert(ctx, "1234", Upsert, alice); err != nil {
		return xray.New(err)
	}

	query = func(name *string, cus *Customer) Query {
		return Query{Slice(0, 100)}
	}
	patch := func(cus *Customer) Patch {
		return Patch{
			Set(&cus.Age, 22),
		}
	}
	count, err := DB.Customers.Update(ctx, query, patch)
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

	found = false
	for id, cus := range DB.Customers.Search(ctx, query, &err) {
		if id == "1234" && cus.Name == "Alice" && cus.Age == 22 {
			found = true
		}
	}
	if err != nil {
		return xray.New(err)
	}
	if !found {
		return fmt.Errorf("expected to find alice")
	}

	var counter atomic.Int32
	stats := func(name *string, cus *Customer) Stats {
		return Stats{
			Count(&counter),
		}
	}
	if err := DB.Customers.Output(ctx, nil, stats); err != nil {
		return xray.New(err)
	}
	if counter.Load() != 2 {
		return fmt.Errorf("expected 2 customers, got %v", counter.Load())
	}

	existed, err := DB.Customers.Delete(ctx, "1234", nil)
	if err != nil {
		return xray.New(err)
	}
	if !existed {
		return fmt.Errorf("expected to delete alice")
	}

	if err := testComposites(ctx, db); err != nil {
		return xray.New(err)
	}
	if err := testValuers(ctx, db); err != nil {
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
	DB := Open[struct {
		Composites Map[Index, Record] `sql:"testing_composites"`
	}](db)
	var (
		index = Index{"a", "b"}
	)
	_, err := DB.Composites.UnsafeDelete(ctx, func(*Index, *Record) Query {
		return Query{Slice(0, 100)}
	})
	if err != nil {
		return xray.New(err)
	}
	if err := DB.Composites.Insert(ctx, index, Create, Record{Value: 1}); err != nil {
		return xray.New(err)
	}
	val, ok, err := DB.Composites.Lookup(ctx, index)
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

func testValuers(ctx context.Context, db Database) error {
	type Value xyz.Switch[string, struct {
		Hello Value `json:"hello"`
		World Value `json:"world"`
	}]
	var Values = xyz.AccessorFor(Value.Values)
	DB := Open[struct {
		Switches Map[string, Value] `sql:"testing_switch"`
	}](db)
	_, err := DB.Switches.UnsafeDelete(ctx, func(*string, *Value) Query {
		return Query{Slice(0, 100)}
	})
	if err != nil {
		return xray.New(err)
	}
	if err := DB.Switches.Insert(ctx, "1234", Create, Values.World); err != nil {
		return xray.New(err)
	}
	check, ok, err := DB.Switches.Lookup(ctx, "1234")
	if err != nil {
		return xray.New(err)
	}
	if !ok {
		return fmt.Errorf("expected to find record")
	}
	if check != Values.World {
		return fmt.Errorf("expected world, got %v", check)
	}
	return nil
}
