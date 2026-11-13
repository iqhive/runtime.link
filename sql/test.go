package sql

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/iqhive/runtime.link/api/xray"
	"github.com/iqhive/runtime.link/sql/std/sodium"
	"github.com/iqhive/runtime.link/xyz"
)

// Test the implementation of a [Database] against the SODIUM specification.
// This function creates new 'testing_' prefixed tables in the database. Each
// part of the test starts by deleting up to 100 records from its table, then
// leaves the records it creates in the database, whether it passes or fails,
// to assist with debugging. A [Database] that is a [sodium.Committer] is also
// tested for how it commits, and for each expression that limits a query.
func Test(ctx context.Context, db Database) error {
	type Customer struct {
		Name string
		Age  int
	}
	DB := Open[struct {
		Customers Map[string, Customer] `sql:"testing_customers"`
	}](db)

	// Every write below waits for a [sodium.Committer] to answer its commit,
	// so one that does not is found first, with an error that says so.
	if sodium.AnswersCommits(db) {
		if err := testCommitAnswered(ctx, db, commitAnswerWait); err != nil {
			return xray.New(err)
		}
	}

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
	if err := DB.Customers.Insert(ctx, "1234", Create, bob); !errors.Is(err, ErrDuplicate) {
		return fmt.Errorf("expected ErrDuplicate creating over a different record, got %v", err)
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

	tests := []func(context.Context, Database) error{testComposites, testValuers, testScopes}
	if sodium.AnswersCommits(db) {
		tests = append(tests, testFilters, testTransactions)
	}
	for _, test := range tests {
		if err := test(ctx, db); err != nil {
			return xray.New(err)
		}
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

func testTransactions(ctx context.Context, db Database) error {
	DB := Open[struct {
		Entries Map[string, int] `sql:"testing_transactions"`
	}](db)
	entries := &DB.Entries
	_, err := entries.UnsafeDelete(ctx, func(*string, *int) Query {
		return Query{Slice(0, 100)}
	})
	if err != nil {
		return xray.New(err)
	}
	failure := errors.New("failed")
	err = entries.Manage(ctx, 0, func(ctx context.Context) error {
		if err := entries.Insert(ctx, "failed", Create, 1); err != nil {
			return err
		}
		return failure
	})
	if !errors.Is(err, failure) {
		return fmt.Errorf("expected a failed transaction to return its failure, got %v", err)
	}
	err = entries.Manage(ctx, 0, func(ctx context.Context) error {
		return entries.Insert(ctx, "committed", Create, 2)
	})
	if err != nil {
		return xray.New(err)
	}
	if entries.db != nil {
		if err := abandonTransaction(ctx, entries.db, entries.to); err != nil {
			return xray.New(err)
		}
	}
	// A write that was reported as made stays made when its caller is done
	// with the context, as any caller with a deadline is on its way out.
	done, cancel := context.WithCancel(ctx)
	err = entries.Insert(done, "reported", Create, 4)
	cancel()
	if err != nil {
		return xray.New(err)
	}
	for _, entry := range []struct {
		key  string
		kept bool
	}{{"failed", false}, {"committed", true}, {"abandoned", false}, {"reported", true}} {
		_, found, err := entries.Lookup(ctx, entry.key)
		if err != nil {
			return xray.New(err)
		}
		if found != entry.kept {
			return fmt.Errorf("expected the write of the %v transaction to be kept: %v, got %v", entry.key, entry.kept, found)
		}
	}
	return nil
}

// abandonTransaction writes the 'abandoned' entry in a transaction that is
// closed without a commit, as one is by a caller that timed out or failed,
// which a [Database] has to roll back. A [Map] says that it is rolling back
// first when it can, so the database is spoken to directly.
func abandonTransaction(ctx context.Context, db Database, table sodium.Table) error {
	jobs, err := db.Manage(ctx, 0)
	if err != nil {
		return err
	}
	defer close(jobs)
	insert := db.Insert(table, ValuesOf("abandoned"), bool(Create), nil, ValuesOf(3))
	select {
	case jobs <- insert:
	case <-ctx.Done():
		return ctx.Err()
	}
	_, err = insert.Wait(ctx)
	return err
}

// commitAnswerWait is how long [Test] waits for a [Database] to say how the
// commit of a transaction with nothing in it went. That takes one round trip,
// so a database that has not answered by then is taken not to answer at all.
const commitAnswerWait = 10 * time.Second

// testCommitAnswered checks that a [sodium.Commit] is answered, which a [Map]
// waits for before it reports a write as made. The database is spoken to
// directly: a [Map] would wait for as long as the context lasts.
func testCommitAnswered(ctx context.Context, db Database, wait time.Duration) error {
	if db == nil || db == &stub {
		return nil // a Map without a Database commits nothing.
	}
	jobs, err := db.Manage(ctx, 0)
	if err != nil {
		return err
	}
	defer close(jobs)
	commit := sodium.NewCommit()
	select {
	case jobs <- commit:
	case <-ctx.Done():
		return ctx.Err()
	}
	answer, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	if _, err := commit.Wait(answer); err != nil {
		if ctx.Err() == nil && answer.Err() != nil {
			return fmt.Errorf("expected the commit of a transaction to be answered with sodium.Commit.Done, it was not within %v", wait)
		}
		return fmt.Errorf("expected the commit of an empty transaction to be made, got %w", err)
	}
	return nil
}

// testFilters checks each expression that limits a query to some of the rows,
// in a search and in a write: a database that takes one for every row would
// delete or update what it was told to leave alone.
func testFilters(ctx context.Context, db Database) error {
	type Person struct {
		Name string
		Age  int
		Note string
	}
	DB := Open[struct {
		People Map[string, Person] `sql:"testing_filters"`
	}](db)
	all := func(*string, *Person) Query { return Query{Slice(0, 100)} }
	if _, err := DB.People.UnsafeDelete(ctx, all); err != nil {
		return xray.New(err)
	}
	for id, person := range map[string]Person{
		"a": {Name: "adam", Age: 5},
		"b": {Name: "beth", Age: 18, Note: "noted"},
		"c": {Name: "adele", Age: 50},
	} {
		if err := DB.People.Insert(ctx, id, Create, person); err != nil {
			return xray.New(err)
		}
	}
	for _, filter := range []struct {
		name string
		expr func(*Person) sodium.Expression
		want string
	}{
		{"Where.Min", func(p *Person) sodium.Expression { return Where(&p.Age).Min(18) }, "bc"},
		{"Where.Max", func(p *Person) sodium.Expression { return Where(&p.Age).Max(18) }, "ab"},
		{"Where.MoreThan", func(p *Person) sodium.Expression { return Where(&p.Age).MoreThan(18) }, "c"},
		{"Where.LessThan", func(p *Person) sodium.Expression { return Where(&p.Age).LessThan(18) }, "a"},
		{"Match.HasPrefix", func(p *Person) sodium.Expression { return Match(&p.Name).HasPrefix("ad") }, "ac"},
		{"Match.HasSuffix", func(p *Person) sodium.Expression { return Match(&p.Name).HasSuffix("th") }, "b"},
		{"Match.Contains", func(p *Person) sodium.Expression { return Match(&p.Name).Contains("de") }, "c"},
		{"Empty", func(p *Person) sodium.Expression { return Empty(&p.Note) }, "ac"},
		{"Avoid(Empty)", func(p *Person) sodium.Expression { return Avoid(Empty(&p.Note)) }, "b"},
		{"Avoid(Where.Min)", func(p *Person) sodium.Expression { return Avoid(Where(&p.Age).Min(18)) }, "a"},
		{"Avoid(Match.HasPrefix)", func(p *Person) sodium.Expression { return Avoid(Match(&p.Name).HasPrefix("ad")) }, "b"},
		{"Cases", func(p *Person) sodium.Expression {
			return Cases(Index(&p.Name).Equals("adam"), Where(&p.Age).MoreThan(18))
		}, "ac"},
		{"Merge", func(p *Person) sodium.Expression {
			return Merge(Match(&p.Name).HasPrefix("ad"), Where(&p.Age).MoreThan(18))
		}, "c"},
	} {
		query := func(_ *string, p *Person) Query { return Query{filter.expr(p)} }
		var err error
		var found []string
		for id := range DB.People.Search(ctx, query, &err) {
			found = append(found, id)
		}
		if err != nil {
			return xray.New(err)
		}
		slices.Sort(found)
		if got := strings.Join(found, ""); got != filter.want {
			return fmt.Errorf("expected %v to find the records %q, got %q", filter.name, filter.want, got)
		}
		count, err := DB.People.Count(ctx, query)
		if err != nil {
			return xray.New(err)
		}
		if count != len(filter.want) {
			return fmt.Errorf("expected %v to count %v records, got %v", filter.name, len(filter.want), count)
		}
	}
	deleted, err := DB.People.UnsafeDelete(ctx, func(_ *string, p *Person) Query {
		return Query{Where(&p.Age).LessThan(18), Slice(0, 100)}
	})
	if err != nil {
		return xray.New(err)
	}
	if deleted != 1 {
		return fmt.Errorf("expected a delete limited by Where to delete 1 record, got %v", deleted)
	}
	deleted, err = DB.People.UnsafeDelete(ctx, func(_ *string, p *Person) Query {
		return Query{Match(&p.Name).HasPrefix("be"), Slice(0, 100)}
	})
	if err != nil {
		return xray.New(err)
	}
	if deleted != 1 {
		return fmt.Errorf("expected a delete limited by Match to delete 1 record, got %v", deleted)
	}
	if _, kept, err := DB.People.Lookup(ctx, "c"); err != nil || !kept {
		return fmt.Errorf("expected the record that no delete asked for to be kept, got %v %v", kept, err)
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
