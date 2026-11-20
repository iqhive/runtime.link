package sql

import (
	"context"
	"testing"

	"github.com/iqhive/runtime.link/sql/std/sodium"
	"github.com/iqhive/runtime.link/xyz"
)

// checkedPatch is where a patch meets a scope. A modification that names a
// column of the value is checked against what the scope grants, and one that
// names a column of the index is refused: it rewrites the key of the row,
// which moves it, onto a key that may be another row's and one that no scope
// grants, even when the value it would move the key onto is one the scope
// holds.
func TestCheckedPatchRefusesAModificationOfAKey(t *testing.T) {
	type key struct {
		Tenant string `sql:"tenant,scoped"`
		ID     int
	}
	type doc struct {
		Text string
		Team string `sql:"team,scoped"`
	}
	ctx := Scope(Scope(context.Background(), "tenant", "a", "b"), "team", "red")
	scope, err := scopeFor[key, doc](ctx)
	if err != nil {
		t.Fatal(err)
	}
	keyField, valField := fieldsOf[key, doc]("id")
	to := sodium.Table{
		Name:  "docs",
		Index: columnsOf(keyField),
		Value: columnsOf(valField),
	}
	index := sentinals.index.assert(to.Name, keyField, new(key), to.Index).(*key)
	value := sentinals.value.assert(to.Name, valField, new(doc), to.Value).(*doc)
	set := func(column sodium.Column, val string) sodium.Modification {
		return sodium.Modifications.Set.As(xyz.NewPair(column, sodium.Values.String.As(val)))
	}
	kept := func(t *testing.T, name string, mod sodium.Modification) {
		t.Helper()
		checked, ok := scope.checkedPatch(index, value, to, Patch{mod})
		if !ok || len(checked) != 1 {
			t.Errorf("%s: gave %v and %v, want the one modification kept", name, checked, ok)
		}
	}
	refused := func(t *testing.T, name string, mod sodium.Modification) {
		t.Helper()
		if checked, ok := scope.checkedPatch(index, value, to, Patch{mod}); ok {
			t.Errorf("%s: gave %v, want it refused", name, checked)
		}
	}

	// A modification of the scoped column of the key, onto a value that the
	// scope grants, is refused all the same.
	refused(t, "the scoped column of the key", set(to.Index[0], "b"))
	// A modification of a column of the key that no scope is named for, with
	// any value at all, is refused.
	refused(t, "a column of the key that no scope is for", set(to.Index[1], "2"))
	// A modification of a column of the value that no scope is named for is
	// checked as it always was.
	kept(t, "a column of the value that no scope is for", set(to.Value[0], "kept"))
	// A modification of the scoped column of the value, onto a value that the
	// scope grants, is kept.
	kept(t, "a granted value of the scope", set(to.Value[1], "red"))
	// One onto a value that the scope does not grant is refused.
	refused(t, "a value out of the scope", set(to.Value[1], "blue"))
	// A column that the table does not have at all is refused.
	refused(t, "a column the table does not have", set(sodium.Column{Name: "none"}, "x"))
	// A modification wrapped in the array that [Set] returns for a value of
	// more than one column is refused by what it holds.
	refused(t, "a modification of the key inside an array", sodium.Modifications.Arr.As(
		[]sodium.Modification{set(to.Value[0], "kept"), set(to.Index[0], "b")},
	))
}
