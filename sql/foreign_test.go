package sql_test

import (
	"strings"
	"testing"

	"github.com/iqhive/runtime.link/sql"
	"github.com/iqhive/runtime.link/sql/std/sodium"
)

type pet struct {
	Label  string
	Weight int
}

type zoo struct {
	Accounts sql.Map[string, account] `sql:"zoo_accounts"`
	Pets     sql.Map[string, pet]     `sql:"zoo_pets"`
}

// petOf returns what the functions of the pets of a zoo are given, which is
// what an expression on a pet is built from.
func petOf(t *testing.T, DB *zoo) *pet {
	t.Helper()
	var given *pet
	var issue error
	for range DB.Pets.Search(testContext(t), func(_ *string, p *pet) sql.Query { given = p; return nil }, &issue) {
	}
	if issue != nil || given == nil {
		t.Fatalf("the pets were searched with %v, %v", given, issue)
	}
	return given
}

// strangers are the expressions there are, each on a field of a pet.
func strangers(p *pet) map[string]sodium.Expression {
	return map[string]sodium.Expression{
		"Index":  sql.Index(&p.Label).Equals("rex"),
		"Where":  sql.Where(&p.Weight).Min(1),
		"Match":  sql.Match(&p.Label).HasPrefix("r"),
		"Order":  sql.Order(&p.Weight).Decreasing(),
		"Empty":  sql.Empty(&p.Label),
		"Avoid":  sql.Avoid(sql.Index(&p.Label).Equals("rex")),
		"Cases":  sql.Cases(sodium.Expressions.Value.As(false), sql.Where(&p.Weight).LessThan(1)),
		"Merge":  sql.Merge(sql.Empty(&p.Weight), sql.Index(&p.Label).Equals("rex")),
		"Nested": sql.Avoid(sql.Cases(sql.Merge(sql.Match(&p.Label).Contains("e")))),
	}
}

// operationsWith are the operations that take a query or a check, each given
// one expression and reporting its error.
func operationsWith(DB *zoo, expr sodium.Expression) map[string]func(*testing.T) error {
	query := func(*string, *account) sql.Query { return sql.Query{expr, sql.Slice(0, 10)} }
	check := func(*account) sql.Check { return sql.Check{expr} }
	older := func(a *account) sql.Patch { a.Age++; return nil }
	return map[string]func(*testing.T) error{
		"Search": func(t *testing.T) error {
			var issue error
			for key := range DB.Accounts.Search(testContext(t), query, &issue) {
				t.Errorf("the search gave the row %s", key)
			}
			return issue
		},
		"First": func(t *testing.T) error { _, _, _, err := DB.Accounts.First(testContext(t), query); return err },
		"Count": func(t *testing.T) error { _, err := DB.Accounts.Count(testContext(t), query); return err },
		"UnsafeDelete": func(t *testing.T) error {
			_, err := DB.Accounts.UnsafeDelete(testContext(t), query)
			return err
		},
		"Update": func(t *testing.T) error { _, err := DB.Accounts.Update(testContext(t), query, older); return err },
		"Delete": func(t *testing.T) error { _, err := DB.Accounts.Delete(testContext(t), "a", check); return err },
		"Mutate": func(t *testing.T) error { _, err := DB.Accounts.Mutate(testContext(t), "a", check, older); return err },
	}
}

// A Map without a database goes by the values it holds, and an expression on
// the column of a database says nothing of them: it would match every value,
// which is what a delete then deletes. A write with one is refused instead,
// and a read goes on as it always has.
func TestExpressionOnColumnForMapWithoutDatabase(t *testing.T) {
	given := petOf(t, sql.Open[zoo](newMemoryDatabase()))
	stored := account{Name: "a", Age: 3}
	for kind, expr := range strangers(given) {
		DB := sql.Open[zoo](sql.New())
		if err := DB.Accounts.Set(testContext(t), "a", stored); err != nil {
			t.Fatal(err)
		}
		for name, operation := range operationsWith(DB, expr) {
			switch name {
			case "Search", "First", "Count":
				continue
			}
			err := operation(t)
			if err == nil || !strings.Contains(err.Error(), "another Map") {
				t.Errorf("%s with an expression (%s) on a column of a database gave: %v", name, kind, err)
			}
		}
		if got, err := DB.Accounts.Get(testContext(t), "a"); got != stored || err != nil {
			t.Errorf("after the operations with %s the account is %+v, %v", kind, got, err)
		}
	}
}

// What a Map without a database is asked for a value is still asked: only an
// expression that could not be worked out from a value is refused.
func TestExpressionsWorkedOutFromValuesAreKept(t *testing.T) {
	var m sql.Map[string, account]
	ctx := testContext(t)
	for name, age := range map[string]int{"adam": 5, "beth": 18, "adele": 50} {
		if err := m.Set(ctx, name, account{Name: name, Age: age}); err != nil {
			t.Fatal(err)
		}
	}
	n, err := m.Count(ctx, func(k *string, a *account) sql.Query {
		return sql.Query{
			sql.Avoid(sql.Index(k).Equals("beth")),
			sql.Cases(sql.Where(&a.Age).Min(18), sql.Match(&a.Name).HasSuffix("am")),
			sql.Merge(sql.Avoid(sql.Empty(&a.Name)), sql.Match(&a.Name).HasPrefix("ad")),
			sql.Order(&a.Age).Decreasing(),
			sql.Slice(0, 10),
		}
	})
	if n != 2 || err != nil {
		t.Errorf("counted %d, %v", n, err)
	}
	deleted, err := m.Delete(ctx, "adam", func(a *account) sql.Check { return sql.Check{sql.Where(&a.Age).LessThan(18)} })
	if !deleted || err != nil {
		t.Errorf("a delete with a check gave %v and %v", deleted, err)
	}
}
