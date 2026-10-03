package sql

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/iqhive/runtime.link/api/xray"
	"github.com/iqhive/runtime.link/xyz"
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
		return xray.New(fmt.Errorf("expected 2 customers, got %v", count))
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
		return xray.New(fmt.Errorf("expected 2 customers, got %v", counter.Load()))
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
	if err := testScopes(ctx, db); err != nil {
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

func testScopes(ctx context.Context, db Database) error {
	type Document struct {
		Title  string
		Team   string `sql:"team,scoped"`
		Region string `sql:"region,scoped"`
	}
	DB := Open[struct {
		Documents Map[string, Document] `sql:"testing_scoped_documents"`
	}](db)
	admin := Admin(ctx, "testing")
	_, err := DB.Documents.UnsafeDelete(admin, func(*string, *Document) Query {
		return Query{Slice(0, 100)}
	})
	if err != nil {
		return xray.New(err)
	}
	for id, doc := range map[string]Document{
		"r1": {Title: "one", Team: "red", Region: "north"},
		"r2": {Title: "two", Team: "red", Region: "south"},
		"b1": {Title: "three", Team: "blue", Region: "north"},
	} {
		if err := DB.Documents.Insert(admin, id, Create, doc); err != nil {
			return xray.New(err)
		}
	}
	all := func(*string, *Document) Query { return Query{Slice(0, 100)} }
	ids := func(ctx context.Context) (map[string]bool, error) {
		var err error
		found := make(map[string]bool)
		for id := range DB.Documents.Search(ctx, all, &err) {
			found[id] = true
		}
		return found, err
	}

	if _, err := DB.Documents.Count(ctx, nil); !errors.Is(err, ErrUnauthorized) {
		return fmt.Errorf("expected ErrUnauthorized counting without a scope, got %v", err)
	}
	if _, err := ids(Scope(ctx, "team", "red")); !errors.Is(err, ErrUnauthorized) {
		return fmt.Errorf("expected ErrUnauthorized searching with only one of two scopes, got %v", err)
	}

	red := Widen[string](Scope(ctx, "team", "red"), "region")
	found, err := ids(red)
	if err != nil {
		return xray.New(err)
	}
	if len(found) != 2 || !found["r1"] || !found["r2"] {
		return fmt.Errorf("expected the red team's documents, got %v", found)
	}
	count, err := DB.Documents.Count(red, nil)
	if err != nil {
		return xray.New(err)
	}
	if count != 2 {
		return fmt.Errorf("expected to count 2 red documents, got %v", count)
	}
	if _, ok, err := DB.Documents.Lookup(red, "b1"); err != nil || ok {
		return fmt.Errorf("expected a blue document to be not found, got %v %v", ok, err)
	}
	redNorth := Scope(red, "region", "north")
	found, err = ids(redNorth)
	if err != nil {
		return xray.New(err)
	}
	if len(found) != 1 || !found["r1"] {
		return fmt.Errorf("expected only the red team's northern document, got %v", found)
	}
	if found, err := ids(Scope(red, "team", "blue")); err != nil || len(found) != 0 {
		return fmt.Errorf("expected scoping red to blue to match nothing, got %v %v", found, err)
	}

	blue := Document{Title: "four", Team: "blue", Region: "north"}
	if err := DB.Documents.Insert(red, "b2", Create, blue); !errors.Is(err, ErrAccessDenied) {
		return fmt.Errorf("expected ErrAccessDenied inserting a blue document, got %v", err)
	}
	if err := DB.Documents.Insert(red, "b1", Upsert, Document{Title: "taken", Team: "red"}); !errors.Is(err, ErrAccessDenied) {
		return fmt.Errorf("expected ErrAccessDenied upserting over a blue document, got %v", err)
	}
	if doc, err := DB.Documents.Get(admin, "b1"); err != nil || doc.Title != "three" || doc.Team != "blue" {
		return fmt.Errorf("expected the blue document to be untouched, got %v %v", doc, err)
	}
	if err := DB.Documents.Insert(red, "r1", Upsert, Document{Title: "uno", Team: "red", Region: "north"}); err != nil {
		return xray.New(err)
	}
	if err := DB.Documents.Insert(red, "r3", Upsert, Document{Title: "tres", Team: "red", Region: "north"}); err != nil {
		return xray.New(err)
	}
	if doc, err := DB.Documents.Get(red, "r1"); err != nil || doc.Title != "uno" {
		return fmt.Errorf("expected upsert to update a red document, got %v %v", doc, err)
	}
	if err := DB.Documents.Insert(red, "r1", Upsert, blue); !errors.Is(err, ErrAccessDenied) {
		return fmt.Errorf("expected ErrAccessDenied upserting a red document to blue, got %v", err)
	}
	if err := DB.Documents.Insert(red, "b3", Upsert, blue); !errors.Is(err, ErrAccessDenied) {
		return fmt.Errorf("expected ErrAccessDenied upserting a new blue document, got %v", err)
	}
	if doc, err := DB.Documents.Get(admin, "r1"); err != nil || doc.Title != "uno" || doc.Team != "red" {
		return fmt.Errorf("expected the red document to be untouched, got %v %v", doc, err)
	}

	updated, err := DB.Documents.Update(red, all, func(doc *Document) Patch {
		return Patch{Set(&doc.Title, "updated")}
	})
	if err != nil {
		return xray.New(err)
	}
	if updated != 3 {
		return fmt.Errorf("expected to update the 3 red documents, got %v", updated)
	}
	if doc, err := DB.Documents.Get(admin, "b1"); err != nil || doc.Title != "three" {
		return fmt.Errorf("expected the blue document to be untouched by update, got %v %v", doc, err)
	}
	if _, err := DB.Documents.Mutate(red, "r1", nil, func(doc *Document) Patch {
		return Patch{Set(&doc.Team, "blue")}
	}); !errors.Is(err, ErrAccessDenied) {
		return fmt.Errorf("expected ErrAccessDenied moving a document to the blue team, got %v", err)
	}
	if doc, err := DB.Documents.Get(admin, "r1"); err != nil || doc.Team != "red" {
		return fmt.Errorf("expected the document to stay on the red team, got %v %v", doc, err)
	}

	if deleted, err := DB.Documents.Delete(red, "b1", nil); err != nil || deleted {
		return fmt.Errorf("expected not to delete a blue document, got %v %v", deleted, err)
	}
	if deleted, err := DB.Documents.UnsafeDelete(red, all); err != nil || deleted != 3 {
		return fmt.Errorf("expected to delete the 3 red documents, got %v %v", deleted, err)
	}
	if found, err := ids(admin); err != nil || len(found) != 1 || !found["b1"] {
		return fmt.Errorf("expected only the blue document to remain, got %v %v", found, err)
	}
	return nil
}
