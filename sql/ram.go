package sql

import (
	"context"
	"iter"
	"math"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/iqhive/runtime.link/api/xray"
	"github.com/iqhive/runtime.link/sql/std/sodium"
	"github.com/iqhive/runtime.link/xyz"
)

var stub struct {
	Database
}

func New() Database { return &stub }

type ram[K comparable, V any] struct {
	sync.Mutex // held by every write to Map.

	// Map holds a *V for each key. A row is replaced rather than written to,
	// so its pointer tells versions of the row apart whatever V is.
	Map sync.Map

	id atomic.Uint64
}

// store a row, the caller must hold the lock.
func (r *ram[K, V]) store(ctx context.Context, key K, value V) {
	prior, _ := r.Map.Swap(key, &value)
	r.remember(ctx, key, prior, &value)
}

// erase a row, the caller must hold the lock.
func (r *ram[K, V]) erase(ctx context.Context, key K) {
	prior, _ := r.Map.LoadAndDelete(key)
	r.remember(ctx, key, prior, nil)
}

// remember a write in the transaction held by the context, if there is one.
func (r *ram[K, V]) remember(ctx context.Context, key K, prior, wrote any) {
	tx, ok := ctx.Value(ramTransactionKey{}).(*ramTransaction)
	if !ok {
		return
	}
	tx.record(func() { r.undo(key, prior, wrote) })
}

// undo a write, unless the row has since been written outside of the
// transaction, as that write was acknowledged and is not the transaction's
// to take back.
func (r *ram[K, V]) undo(key K, prior, wrote any) {
	r.Lock()
	defer r.Unlock()
	current, _ := r.Map.Load(key)
	if current != wrote {
		return
	}
	if prior == nil {
		r.Map.Delete(key)
		return
	}
	r.Map.Store(key, prior)
}

// claim stores a row at a key that is not in use, reporting whether it did.
func (r *ram[K, V]) claim(ctx context.Context, key K, value V) bool {
	r.Lock()
	defer r.Unlock()
	if _, exists := r.Map.Load(key); exists {
		return false
	}
	r.store(ctx, key, value)
	return true
}

type ramTransactionKey struct{}

const errTransactionBusy = errorString("another transaction holds the lock on RAM maps, likely a nested Manage given the outer context rather than the one passed to fn")

// ramTransactionWait limits the wait for transactionLock. A nested call given
// the outer context is waiting for its own transaction, so if that context has
// no deadline, nothing else would ever end the wait.
const ramTransactionWait = time.Minute

// ramTransaction is an undo log for writes to any RAM map made with its context.
type ramTransaction struct {
	sync.Mutex
	undo []func()
}

func (tx *ramTransaction) record(undo func()) {
	tx.Lock()
	defer tx.Unlock()
	tx.undo = append(tx.undo, undo)
}

func (tx *ramTransaction) rollback() {
	tx.Lock()
	undo := tx.undo
	tx.undo = nil
	tx.Unlock()
	for i := len(undo) - 1; i >= 0; i-- {
		undo[i]()
	}
}

// lockRAMTransactions takes transactionLock, the caller must release it.
func lockRAMTransactions(ctx context.Context, wait time.Duration) error {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case transactionLock <- struct{}{}:
		return nil
	case <-ctx.Done():
		return xray.New(ctx.Err())
	case <-timer.C:
		return xray.New(errTransactionBusy)
	}
}

// manageRAM runs fn as a transaction across every RAM map, one at a time. As
// on a [Database], a nested call joins the transaction held by its context.
func manageRAM(ctx context.Context, wait time.Duration, fn func(context.Context) error) error {
	if _, nested := ctx.Value(ramTransactionKey{}).(*ramTransaction); nested {
		return fn(ctx)
	}
	if err := lockRAMTransactions(ctx, wait); err != nil {
		return err
	}
	defer func() { <-transactionLock }()
	tx := new(ramTransaction)
	commit := false
	defer func() {
		if !commit {
			tx.rollback() // Roll back on callback failure or panic.
		}
	}()
	err := fn(context.WithValue(ctx, ramTransactionKey{}, tx))
	commit = err == nil
	return err
}

func (m *ram[K, V]) match(expr []sodium.Expression) bool {
	for _, e := range expr {
		switch xyz.ValueOf(e) {
		case sodium.Expressions.Value:
			if !sodium.Expressions.Value.Get(e) {
				return false
			}
		}
	}
	return true
}

func (r *ram[K, V]) Add(ctx context.Context, value V) (K, error) {
	var key K
	if randomized, ok := any(&key).(interface {
		Randomize()
	}); ok {
		randomized.Randomize()
		if !r.claim(ctx, key, value) {
			return key, ErrDuplicate
		}
		return key, nil
	}
	rkey := reflect.ValueOf(&key).Elem()
	for {
		switch rkey.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			id := int64(r.id.Add(1))
			if rkey.OverflowInt(id) {
				return [1]K{}[0], ErrDuplicate
			}
			rkey.SetInt(id)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			id := r.id.Add(1)
			if rkey.OverflowUint(id) {
				return [1]K{}[0], ErrDuplicate
			}
			rkey.SetUint(id)
		default:
			return key, ErrInsertOnly
		}
		if r.claim(ctx, key, value) {
			return key, nil
		}
	}
}

func (r *ram[K, V]) Insert(ctx context.Context, key K, flag Flag, value V) error {
	r.Lock()
	defer r.Unlock()
	existing, exists := r.Map.Load(key)
	if !exists || bool(flag) {
		r.store(ctx, key, value)
		return nil
	}
	if !reflect.DeepEqual(*existing.(*V), value) {
		return ErrDuplicate
	}
	return nil
}

// Upsert stores the value at key, unless a value exists there that does not
// pass the check.
func (m *ram[K, V]) Upsert(ctx context.Context, key K, check CheckFunc[V], value V) bool {
	m.Lock()
	defer m.Unlock()
	if ival, exists := m.Map.Load(key); exists {
		val := *ival.(*V)
		if !m.match(check(&val)) {
			return false
		}
	}
	m.store(ctx, key, value)
	return true
}

func (m *ram[K, V]) Output(ctx context.Context, query QueryFunc[K, V], stats StatsFunc[K, V]) error {
	if stats == nil {
		return nil
	}
	var zeroKey K
	var zeroVal V
	for _, stat := range stats(&zeroKey, &zeroVal) {
		stat.reset()
	}
	m.Map.Range(func(ikey, ival any) bool {
		key := ikey.(K)
		val := *ival.(*V)
		if query != nil && !m.match(query(&key, &val)) {
			return true
		}
		stats(&key, &val)
		return true
	})
	return nil
}

func (m *ram[K, V]) UnsafeDelete(ctx context.Context, query QueryFunc[K, V]) (int, error) {
	m.Lock()
	defer m.Unlock()
	var count int
	var err error
	for key := range m.Search(ctx, query, &err) {
		m.erase(ctx, key)
		count++
	}
	return count, err
}

func (m *ram[K, V]) Update(ctx context.Context, query QueryFunc[K, V], patch PatchFunc[V]) (int, error) {
	var count int
	var err error
	for key := range m.Search(ctx, query, &err) {
		patched, failure := m.patchRow(ctx, key, query, patch)
		if failure != nil {
			return count, failure
		}
		if patched {
			count++
		}
	}
	return count, err
}

// patchRow runs the patch without the lock, as it is the caller's code and an
// update can touch many rows. The result is kept only if the row is still the
// version that was patched, otherwise the row is looked at again: it may have
// been deleted, or stopped matching, and if not the patch needs its new value.
func (m *ram[K, V]) patchRow(ctx context.Context, key K, query QueryFunc[K, V], patch PatchFunc[V]) (bool, error) {
	for {
		seen, exists := m.Map.Load(key)
		if !exists {
			return false, nil
		}
		val := *seen.(*V)
		if !m.match(query(&key, &val)) {
			return false, nil
		}
		patch(&val)
		if m.replace(ctx, key, seen, val) {
			return true, nil
		}
		if err := ctx.Err(); err != nil {
			return false, xray.New(err)
		}
	}
}

// replace stores a row if the version at key is the one that was seen.
func (m *ram[K, V]) replace(ctx context.Context, key K, seen any, value V) bool {
	m.Lock()
	defer m.Unlock()
	if current, _ := m.Map.Load(key); current != seen {
		return false
	}
	m.store(ctx, key, value)
	return true
}

func (m *ram[K, V]) Mutate(ctx context.Context, key K, check CheckFunc[V], patch PatchFunc[V]) (bool, error) {
	m.Lock()
	defer m.Unlock()
	ival, exists := m.Map.Load(key)
	if !exists {
		return false, nil
	}
	val := *ival.(*V)
	if check != nil && !m.match(check(&val)) {
		return false, nil
	}
	patch(&val)
	m.store(ctx, key, val)
	return true, nil
}

func (m *ram[K, V]) Delete(ctx context.Context, key K, check CheckFunc[V]) (bool, error) {
	m.Lock()
	defer m.Unlock()
	ival, exists := m.Map.Load(key)
	if !exists {
		return false, nil
	}
	val := *ival.(*V)
	if check != nil && !m.match(check(&val)) {
		return false, nil
	}
	m.erase(ctx, key)
	return true, nil
}

func (m *ram[K, V]) Lookup(ctx context.Context, key K) (V, bool, error) {
	ival, exists := m.Map.Load(key)
	if !exists {
		var zero V
		return zero, false, nil
	}
	return *ival.(*V), true, nil
}

func (m *ram[K, V]) Search(ctx context.Context, query QueryFunc[K, V], err *error) iter.Seq2[K, V] {
	*err = nil
	return func(yield func(K, V) bool) {
		var ordering = query.ordered()
		limit := sodium.Range{Upto: math.MaxInt}
		if query != nil {
			var key K
			var val V
			for _, expr := range query(&key, &val) {
				if xyz.ValueOf(expr) == sodium.Expressions.Range {
					limit = sodium.Expressions.Range.Get(expr)
					break
				}
			}
		}
		if limit.Upto <= limit.From {
			return
		}
		count := 0
		visit := func(key K, val V) bool {
			if count >= limit.Upto {
				return false
			}
			index := count
			count++
			if index < limit.From {
				return true
			}
			return yield(key, val) && count < limit.Upto
		}
		type entry struct {
			key K
			val V
		}
		var matching []entry
		m.Map.Range(func(ikey, ival any) bool {
			key := ikey.(K)
			val := *ival.(*V)
			if query != nil && !m.match(query(&key, &val)) {
				return true
			}
			if ordering {
				matching = append(matching, entry{key, val})
				return true
			}
			return visit(key, val)
		})
		if ordering && len(matching) > 0 {
			var sortingKey K
			var sortingVal V
			var orders []exOrdering
			for _, expr := range query(&sortingKey, &sortingVal) {
				if xyz.ValueOf(expr) == sodium.Expressions.Order {
					orders = append(orders, exOrderExpressions.Functional.Get(sodium.Expressions.Order.Get(expr)))
				}
			}
			sort.Slice(matching, func(i, j int) bool {
				for _, order := range orders {
					sortingKey = matching[i].key
					sortingVal = matching[i].val
					order.Load()
					sortingKey = matching[j].key
					sortingVal = matching[j].val
					if order.Less() {
						return true
					}
					order.Load()
					sortingKey = matching[i].key
					sortingVal = matching[i].val
					if order.Less() {
						return false
					}
				}
				return false
			})
			for _, entry := range matching {
				key := entry.key
				val, ok := m.Map.Load(key)
				if ok {
					if !visit(key, *val.(*V)) {
						return
					}
				}
			}
		}
	}
}
