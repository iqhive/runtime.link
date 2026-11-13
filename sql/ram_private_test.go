package sql_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/iqhive/runtime.link/sql"
)

type draft struct {
	Body []byte
	Tags map[string]bool
}

func storedDraft() draft {
	return draft{Body: []byte("stored"), Tags: map[string]bool{"old": true}}
}

func checkStoredDraft(t *testing.T, m *sql.Map[string, draft], what string) {
	t.Helper()
	got, err := m.Get(testContext(t), "a")
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Body) != "stored" || len(got.Tags) != 1 || !got.Tags["old"] {
		t.Errorf("%s changed the row to %q with tags %v, without a write", what, got.Body, got.Tags)
	}
}

// scribble changes a value in place, the way a caller edits what it was given.
func scribble(d draft) {
	copy(d.Body, "edited")
	d.Tags["new"] = true
}

// A value that was read is the caller's to change, and changing it is not a
// write to the row.
func TestRAMValuesReadArePrivate(t *testing.T) {
	ctx := testContext(t)
	for name, read := range map[string]func(m *sql.Map[string, draft]) (draft, error){
		"Get": func(m *sql.Map[string, draft]) (draft, error) { return m.Get(ctx, "a") },
		"Lookup": func(m *sql.Map[string, draft]) (draft, error) {
			got, _, err := m.Lookup(ctx, "a")
			return got, err
		},
		"First": func(m *sql.Map[string, draft]) (draft, error) {
			_, got, _, err := m.First(ctx, nil)
			return got, err
		},
		"Search": func(m *sql.Map[string, draft]) (got draft, issue error) {
			for _, got = range m.Search(ctx, nil, &issue) {
			}
			return got, issue
		},
		"ordered Search": func(m *sql.Map[string, draft]) (got draft, issue error) {
			byKey := func(k *string, _ *draft) sql.Query { return sql.Query{sql.Order(k).Increasing()} }
			for _, got = range m.Search(ctx, byKey, &issue) {
			}
			return got, issue
		},
	} {
		t.Run(name, func(t *testing.T) {
			var m sql.Map[string, draft]
			if err := m.Set(ctx, "a", storedDraft()); err != nil {
				t.Fatal(err)
			}
			got, err := read(&m)
			if err != nil || string(got.Body) != "stored" {
				t.Fatalf("read %q, %v", got.Body, err)
			}
			scribble(got)
			checkStoredDraft(t, &m, "editing a value that was read")
		})
	}
}

// A value that was stored stays the caller's too.
func TestRAMValuesStoredArePrivate(t *testing.T) {
	ctx := testContext(t)
	for name, store := range map[string]func(m *sql.Map[string, draft], value draft) error{
		"Set":    func(m *sql.Map[string, draft], value draft) error { return m.Set(ctx, "a", value) },
		"Create": func(m *sql.Map[string, draft], value draft) error { return m.Insert(ctx, "a", sql.Create, value) },
		"Mutate": func(m *sql.Map[string, draft], value draft) error {
			if err := m.Set(ctx, "a", draft{}); err != nil {
				return err
			}
			_, err := m.Mutate(ctx, "a", nil, func(d *draft) sql.Patch {
				return sql.Patch{sql.Set(&d.Body, value.Body), sql.Set(&d.Tags, value.Tags)}
			})
			return err
		},
		"Update": func(m *sql.Map[string, draft], value draft) error {
			if err := m.Set(ctx, "a", draft{}); err != nil {
				return err
			}
			_, err := m.Update(ctx, func(*string, *draft) sql.Query { return sql.Query{sql.Slice(0, 1)} }, func(d *draft) sql.Patch {
				*d = value
				return nil
			})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			var m sql.Map[string, draft]
			value := storedDraft()
			if err := store(&m, value); err != nil {
				t.Fatal(err)
			}
			scribble(value)
			checkStoredDraft(t, &m, "editing a value after storing it")
		})
	}
	t.Run("Add", func(t *testing.T) {
		var m sql.Map[int, draft]
		value := storedDraft()
		key, err := m.Add(ctx, value)
		if err != nil {
			t.Fatal(err)
		}
		scribble(value)
		if got, err := m.Get(ctx, key); string(got.Body) != "stored" || got.Tags["new"] || err != nil {
			t.Errorf("editing a value after adding it changed the row to %q with tags %v, %v", got.Body, got.Tags, err)
		}
	})
}

// The usual read, edit and write inside a transaction is undone by a rollback,
// which puts back the version of the row from before the write: that version
// must not be the one the caller edited.
func TestManageRAMRollsBackGetEditSet(t *testing.T) {
	ctx := testContext(t)
	var m sql.Map[string, draft]
	if err := m.Set(ctx, "a", storedDraft()); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("changed my mind")
	err := m.Manage(ctx, 0, func(ctx context.Context) error {
		got, err := m.Get(ctx, "a")
		if err != nil {
			return err
		}
		scribble(got)
		if err := m.Set(ctx, "a", got); err != nil {
			return err
		}
		if during, err := m.Get(ctx, "a"); string(during.Body) != "edited" || !during.Tags["new"] || err != nil {
			t.Errorf("inside the transaction the row reads as %q with tags %v, %v", during.Body, during.Tags, err)
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	checkStoredDraft(t, &m, "a transaction that was rolled back")
}

// Each reader has its own copy, so two of them can edit what they read at once.
// Run with -race.
func TestRAMReadersDoNotShareMemory(t *testing.T) {
	ctx := testContext(t)
	var m sql.Map[string, draft]
	if err := m.Set(ctx, "a", storedDraft()); err != nil {
		t.Fatal(err)
	}
	var readers sync.WaitGroup
	for range 4 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for range 100 {
				got, err := m.Get(ctx, "a")
				if err != nil {
					t.Error(err)
					return
				}
				scribble(got)
			}
		}()
	}
	readers.Wait()
	checkStoredDraft(t, &m, "concurrent readers")
}
