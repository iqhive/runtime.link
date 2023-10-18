package sql

import (
	"context"
	"errors"
	"fmt"
	"hash/maphash"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"runtime.link/api/xray"
	"runtime.link/sql/std/sodium"
	"runtime.link/xyz"
)

// This file acts as a bit of a reference piece for runtime.link, and xyz specifically.
// It uses a number of features both of Go and of xyz, so it is a good package to work on
// to get feedback on the design of xyz and runtime.link and its relationship to Go.

// New returns a new [sodium.Database]. It is suitable for use in tests.
func New() sodium.Database {
	ptr := new(atomic.Pointer[chan struct{}])
	end := make(chan struct{})
	close(end)
	ptr.Store(&end)
	var ( // TODO make configurable.
		cpu = runtime.NumCPU()
		max = 100
	)
	var (
		parts = make([]part, cpu)
	)
	ram := dbRAM{
		ptr: ptr,
		cpu: cpu,
		max: max,
		cap: new(int),
		srv: parts,
		job: make(chan (<-chan sodium.Job)),
	}
	parts[0].recv = make(chan *work)
	for i := range parts {
		parts[i].name = i
		parts[i].head = ram
		parts[i].stop = make(chan struct{})
		parts[i].jobs = make([]*work, 0, max)
		parts[i].tabs = make(map[string]*table)
		// link page output to next page input
		if i > 0 {
			parts[i].recv = make(chan *work)
			parts[i-1].send = parts[i].recv
		}
	}
	parts[len(parts)-1].send = parts[0].recv // loop link ends
	return ram
}

// dbRAM is an in-memory reference implementation of [sodium.Database].
type dbRAM struct {
	// The main thread of this database is managed by whichever goroutines is
	// happening to use it. If the database is used by multiple goroutines at
	// once, then jobs are sent to the main thread for processing. The main
	// thread only lives as long as its own jobs are running, when there are
	// other jobs still running, the main thread will be moved to the next
	// available goroutine. In order to become the main thread, a goroutine
	// must place a unique pointer to a context inside this [atomic.Pointer].
	ptr *atomic.Pointer[chan struct{}] // current scheduler

	// When 'cpu' is greater than zero, the database will partition the data
	// within the database across 'cpu' number of threads. While a main thread
	// is running, each partition [page] will spawn a goroutine to process up
	// to [max] jobs in parallel. Each job will progress one row at a time. If
	// no jobs are running, the main thread will shutdown any active goroutines
	// and the [RAM] will scale to zero.
	cpu int // threads
	max int // max jobs per thread.

	cap *int   // number of inserts
	srv []part //each partition of the database runs as a single-threaded goroutine.

	job chan (<-chan sodium.Job) // incoming connections
}

// Dump can be used to debug the contents of the database, it progressively
// prints the contents of the database to [os.Stdout] in an undefined and
// unstable (but human readable) format. Dump is not safe to call while the
// database is in use.
func (db dbRAM) Dump() {
	var tables = make(map[string]struct{})
	for i := range db.srv {
		for name := range db.srv[i].tabs {
			tables[name] = struct{}{}
		}
	}
	for name := range tables {
		fmt.Println(" TABLE", name)
		db.DumpTable(name)
	}
}

// DumpTable can be used to debug the contents of a table, it progressively
// prints the contents of the table to [os.Stdout] in an undefined and
// unstable (but human readable) format. DumpTable is not safe to call
// while the database is in use.
func (db dbRAM) DumpTable(name string) {
	for i := range db.srv {
		fmt.Println(" PART", i)
		db.srv[i].tabs[name].dump()
	}
}

type index map[uint64][]pkey

type pkey struct {
	index int
	value []sodium.Value
}

var seed = maphash.MakeSeed()

func (idx index) lookup(pkey []sodium.Value, cpu int) (sum64 uint64, addr int, part int, ok bool) {
	var hash maphash.Hash
	hash.SetSeed(seed)
	for _, value := range pkey {
		switch xyz.ValueOf(value) {
		case sodium.Values.Bool:
			b := sodium.Values.Bool.Get(value)
			if b {
				hash.WriteByte(1)
			} else {
				hash.WriteByte(0)
			}
		case sodium.Values.Int8:
			i8 := sodium.Values.Int8.Get(value)
			u8 := *(*uint8)(unsafe.Pointer(&i8))
			hash.WriteByte(u8)
		case sodium.Values.Int16:
			i16 := sodium.Values.Int16.Get(value)
			u16 := *(*uint16)(unsafe.Pointer(&i16))
			lo, hi := uint8(u16), uint8(u16>>8)
			hash.WriteByte(lo)
			hash.WriteByte(hi)
		case sodium.Values.Int32:
			i32 := sodium.Values.Int32.Get(value)
			u32 := *(*uint32)(unsafe.Pointer(&i32))
			lo, hi := uint16(u32), uint16(u32>>16)
			hash.WriteByte(uint8(lo))
			hash.WriteByte(uint8(lo >> 8))
			hash.WriteByte(uint8(hi))
			hash.WriteByte(uint8(hi >> 8))
		case sodium.Values.Int64:
			i64 := sodium.Values.Int64.Get(value)
			u64 := *(*uint64)(unsafe.Pointer(&i64))
			lo, hi := uint32(u64), uint32(u64>>32)
			hash.WriteByte(uint8(lo))
			hash.WriteByte(uint8(lo >> 8))
			hash.WriteByte(uint8(lo >> 16))
			hash.WriteByte(uint8(lo >> 24))
			hash.WriteByte(uint8(hi))
			hash.WriteByte(uint8(hi >> 8))
			hash.WriteByte(uint8(hi >> 16))
			hash.WriteByte(uint8(hi >> 24))
		case sodium.Values.Uint8:
			u8 := sodium.Values.Uint8.Get(value)
			hash.WriteByte(u8)
		case sodium.Values.Uint16:
			u16 := sodium.Values.Uint16.Get(value)
			lo, hi := uint8(u16), uint8(u16>>8)
			hash.WriteByte(lo)
			hash.WriteByte(hi)
		case sodium.Values.Uint32:
			u32 := sodium.Values.Uint32.Get(value)
			lo, hi := uint16(u32), uint16(u32>>16)
			hash.WriteByte(uint8(lo))
			hash.WriteByte(uint8(lo >> 8))
			hash.WriteByte(uint8(hi))
			hash.WriteByte(uint8(hi >> 8))
		case sodium.Values.Uint64:
			u64 := sodium.Values.Uint64.Get(value)
			lo, hi := uint32(u64), uint32(u64>>32)
			hash.WriteByte(uint8(lo))
			hash.WriteByte(uint8(lo >> 8))
			hash.WriteByte(uint8(lo >> 16))
			hash.WriteByte(uint8(lo >> 24))
			hash.WriteByte(uint8(hi))
			hash.WriteByte(uint8(hi >> 8))
			hash.WriteByte(uint8(hi >> 16))
			hash.WriteByte(uint8(hi >> 24))
		case sodium.Values.Float32:
			f32 := sodium.Values.Float32.Get(value)
			u32 := *(*uint32)(unsafe.Pointer(&f32))
			hash.WriteByte(uint8(u32))
			hash.WriteByte(uint8(u32 >> 8))
			hash.WriteByte(uint8(u32 >> 16))
			hash.WriteByte(uint8(u32 >> 24))
		case sodium.Values.Float64:
			f64 := sodium.Values.Float64.Get(value)
			u64 := *(*uint64)(unsafe.Pointer(&f64))
			lo, hi := uint32(u64), uint32(u64>>32)
			hash.WriteByte(uint8(lo))
			hash.WriteByte(uint8(lo >> 8))
			hash.WriteByte(uint8(lo >> 16))
			hash.WriteByte(uint8(lo >> 24))
			hash.WriteByte(uint8(hi))
			hash.WriteByte(uint8(hi >> 8))
			hash.WriteByte(uint8(hi >> 16))
			hash.WriteByte(uint8(hi >> 24))
		case sodium.Values.Time:
			i64 := sodium.Values.Time.Get(value).UnixNano()
			u64 := *(*uint64)(unsafe.Pointer(&i64))
			lo, hi := uint32(u64), uint32(u64>>32)
			hash.WriteByte(uint8(lo))
			hash.WriteByte(uint8(lo >> 8))
			hash.WriteByte(uint8(lo >> 16))
			hash.WriteByte(uint8(lo >> 24))
			hash.WriteByte(uint8(hi))
			hash.WriteByte(uint8(hi >> 8))
			hash.WriteByte(uint8(hi >> 16))
			hash.WriteByte(uint8(hi >> 24))
		case sodium.Values.String:
			str := sodium.Values.String.Get(value)
			hash.WriteString(str)
		case sodium.Values.Bytes:
			buf := sodium.Values.Bytes.Get(value)
			hash.Write(buf)
		}
	}
	sum64 = hash.Sum64()
	pkeys := idx[sum64]
	index := -1
	for i := range pkeys {
		if len(pkeys[i].value) == len(pkey) {
			match := true
			for j := range pkey {
				if pkeys[i].value[j] != pkey[j] {
					match = false
					break
				}
			}
			if match {
				index = pkeys[i].index
			}
		}
	}
	return sum64, index, int(sum64 % uint64(cpu)), index >= 0
}

type part struct {
	name int
	live bool           // only readable by the main thread
	stop chan struct{}  // sendable by the main thread
	wait sync.WaitGroup // readable by both parties

	head dbRAM

	recv chan *work
	send chan *work

	tabs map[string]*table
	jobs []*work // parallel
}

type table struct {
	part int
	cpus int

	rows []tx // row-level locks
	next int
	keys index
	bool columns[uint8]
	char columns[int8]
	i16s columns[int16]
	i32s columns[int32]
	i64s columns[int64]
	byte columns[uint8]
	u16s columns[uint16]
	u32s columns[uint32]
	u64s columns[uint64]
	f32s columns[float32]
	f64s columns[float64]
	text columns[string]
}

type page struct {
	name int // table name
	cpus int // number of cpus

	keys map[uint64]int
}

func (c *columns[T]) dump(i int) {
	var keys []string
	for key := range c.names {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		slice := c.slice[c.names[key]].value
		if i < len(slice) {
			fmt.Print("   ", key, ": ", slice[i])
			fmt.Println()
		}
	}
}

func (p *table) dump() {
	if p == nil {
		return
	}
	for i := 0; i < p.next; i++ {
		fmt.Println("  DATA", i)
		p.bool.dump(i)
		p.char.dump(i)
		p.i16s.dump(i)
		p.i32s.dump(i)
		p.i64s.dump(i)
		p.byte.dump(i)
		p.u16s.dump(i)
		p.u32s.dump(i)
		p.u64s.dump(i)
		p.f32s.dump(i)
		p.f64s.dump(i)
		p.text.dump(i)
	}
}

type tx int
type row int

// log entry in the write-ahead log (WAL).
type log struct {
	tx tx
	op op
}

// op within a [wlog].
type op xyz.Switch[any, struct {
	Search xyz.Case[op, sodium.Query]
	Output xyz.Case[op, xyz.Pair[sodium.Query, sodium.Stats]]
	Delete xyz.Case[op, sodium.Query]
	Insert xyz.Case[op, xyz.Trio[[]sodium.Value, bool, []sodium.Value]]
	Update xyz.Case[op, xyz.Pair[sodium.Query, sodium.Patch]]
}]

var ops = xyz.AccessorFor(op.Values)

type valuable interface {
	int8 | int16 | int32 | int64 | uint8 | uint16 | uint32 | uint64 | float32 | float64 | string
}

type column[T valuable] struct {
	index map[T][]int
	value []T
	order []int
}

func (c *column[T]) delete(row int) {
	c.value[row] = c.value[len(c.value)-1]
	c.value = c.value[:len(c.value)-1]
}

func (c *column[T]) empty(row int) bool {
	var zero T
	return c == nil || c.value[row] == zero
}

func (c *column[T]) compare(i int, val T) int {
	if c.value[i] < val {
		return -1
	}
	if c.value[i] > val {
		return 1
	}
	return 0
}

type columns[T valuable] struct {
	mutex sync.RWMutex
	names map[string]int // supports multiple views.
	slice []column[T]
}

// work enters the [RAM] via the 'job' queue and advances through the
// database one partition [page] at a time. When a job is complete, it is
// marked as done via its 'wg' and 'ch' fields and the creator of the job
// is returned ownership of the underlying memory of the struct.
type work struct {
	op op
	in sodium.Table
	ok int                   // which part of the database is our final destination.
	at int                   // progress through the current page.
	id int                   // insert ID or count
	tx tx                    // identifies this job
	er error                 // error
	ch chan<- []sodium.Value // result channel
	wg chan struct{}         // will be closed when done.
	qg querygroup
}

type querygroup struct {
	need int64        // number of parts needed.
	done atomic.Int64 // number of parts completed.
}

func (w *work) complete() {
	if w.ch != nil {
		close(w.ch)
	}
	close(w.wg)
}

type sortgroup struct {
	seen map[int]int // last seen value for each 'part'
	sort []ordering
}

type ordering struct {
	name string
	kind xyz.TypeOf[sodium.Value]
	less bool
}

// Wait for the SQL to complete.
func (w *work) Wait(ctx context.Context) (int, error) {
	select {
	case <-w.wg:
		return w.id, w.er
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// Manage returns a channel that will manage the execution of the given jobs
// within the transaction level specified by the given [Transaction]. Close
// the channel to commit the transaction, or send a nil [Job] to rollback
// the transaction.
func (db dbRAM) Manage(ctx context.Context, level sodium.Transaction) (chan<- sodium.Job, error) {
	if db.ptr == nil {
		return nil, errors.New("sql.RAM is nil")
	}
	if level != 0 {
		return nil, errors.New("sql.RAM does not support transactions yet")
	}
	tx := make(chan sodium.Job)
	me := make(chan struct{})
retry:
	done := db.ptr.Load()
	select {
	case db.job <- tx:
		return tx, nil
	case <-*done:
		if db.ptr.CompareAndSwap(done, &me) {
			go db.run(level) // start the main database thread.
		}
		goto retry
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Search the [Table] for [Value]s that match the given [Query]. Whenever a
// result is found, the corresponding [Pointer] argument is filled with the
// result and the given callback is called. If the callback returns an error,
// the search is aborted and the operation fails.
func (db dbRAM) Search(table sodium.Table, query sodium.Query, write chan<- []sodium.Value) sodium.Job {
	return &work{
		op: ops.Search.As(query),
		in: table,
		wg: make(chan struct{}),
		ch: write,
	}
}

// Output calculates the requested [Stats] for the given table and
// writes them into the respective [Stats] values.
func (db dbRAM) Output(table sodium.Table, query sodium.Query, stats sodium.Stats, write chan<- []sodium.Value) sodium.Job {
	return &work{
		op: ops.Output.As(xyz.NewPair(query, stats)),
		in: table,
		wg: make(chan struct{}),
	}
}

// Delete should remove any records that match the given query from
// the table. A finite [Range] must be specified, if the [Range] is
// empty, the operation will fail.
func (db dbRAM) Delete(table sodium.Table, query sodium.Query) sodium.Job {
	return &work{
		op: ops.Delete.As(query),
		in: table,
		wg: make(chan struct{}),
	}
}

// Insert a [Value] into the table. If the value already exists, the
// flag determines whether the operation should fail (false) or overwrite
// the existing value (true).
func (db dbRAM) Insert(table sodium.Table, index []sodium.Value, flag bool, value []sodium.Value) sodium.Job {
	return &work{
		op: ops.Insert.As(xyz.NewTrio(index, flag, value)),
		in: table,
		wg: make(chan struct{}),
		id: -1,
	}
}

// Update should apply the given patch to each [Value]s in
// the table that matches the given [Query]. A finite [Range]
// must be specified, if the [Range] is empty, the operation will fail.
func (db dbRAM) Update(table sodium.Table, query sodium.Query, patch sodium.Patch) sodium.Job {
	return &work{
		op: ops.Update.As(xyz.NewPair(query, patch)),
		in: table,
		wg: make(chan struct{}),
	}
}

// get returns the table for the given job.
func (p *part) get(job *work) *table {
	tab := p.tabs[job.in.Name]
	if tab == nil {
		tab = new(table)
		tab.part = p.name
		tab.cpus = p.head.cpu
		tab.keys = make(index)
		tab.bool.names = make(map[string]int)
		tab.char.names = make(map[string]int)
		tab.i16s.names = make(map[string]int)
		tab.i32s.names = make(map[string]int)
		tab.i64s.names = make(map[string]int)
		tab.byte.names = make(map[string]int)
		tab.u16s.names = make(map[string]int)
		tab.u32s.names = make(map[string]int)
		tab.u64s.names = make(map[string]int)
		tab.f32s.names = make(map[string]int)
		tab.f64s.names = make(map[string]int)
		tab.text.names = make(map[string]int)
		p.tabs[job.in.Name] = tab
	}
	return tab
}

// run pending jobs as a single threaded routine.
func (db dbRAM) run(level sodium.Transaction) (err error) {
	defer close(*db.ptr.Load())
	var (
		clients []<-chan sodium.Job
		deletes []int
	)
	for {
		select {
		case job := <-db.job:
			clients = append(clients, job)
		default:
			for i, client := range clients {
				select {
				case job, ok := <-client:
					if !ok {
						// TODO commit.
						deletes = append(deletes, i)
						continue
					}
					if job == nil {
						deletes = append(deletes, i)
						return nil
					}
					if work, ok := job.(*work); ok {
						db.dispatch(work)
					}
				default:
					continue
				}
			}
		}
	}
}

// dispatch consumes the job, processing it through the appropriate worker.
// returns true if the job was accepted, false if context was cancelled.
func (db dbRAM) dispatch(job *work) {
	if db.cpu > 1 {
		// fire up any sleeping workers
		// FIXME only wake up workers that are needed (ie. Insert, key-bound searches)
		for i := range db.srv {
			if !db.srv[i].live {
				db.srv[i].wait.Wait()
				db.srv[i].wait.Add(1)
				db.srv[i].live = true
				go db.srv[i].run()
			}
		}
	}
	job.ok = len(db.srv) - 1
	db.srv[0].recv <- job
}

// run processes jobs in a single thread.
func (p *part) run() {
	var stop bool
	var done = make([]bool, cap(p.jobs))
	var clog int
	for {
		if !stop && len(p.jobs) < cap(p.jobs) {
			select {
			case job := <-p.recv:
				p.jobs = append(p.jobs, job)
			case <-p.stop:
				stop = true
			default:
			}
		}
		if stop && len(p.jobs) == 0 {
			p.wait.Done()
			return
		}
		for i, job := range p.jobs {
			if job == nil {
				continue
			}
			tab := p.get(job)
			if done[i] || tab.apply(job) {
				if job.ok == p.name {
					job.complete()
					p.jobs[i] = p.jobs[len(p.jobs)-1]
					p.jobs = p.jobs[:len(p.jobs)-1]
					if done[i] {
						clog--
						done[i] = false
					}
				} else { // pass the job along to the next part of the database.
					if len(p.jobs) < clog {
						select {
						case p.send <- job:
						default:
							clog++
							done[i] = true
							continue
						}
					} else {
						p.send <- job
					}
					p.jobs[i] = p.jobs[len(p.jobs)-1]
					p.jobs = p.jobs[:len(p.jobs)-1]
					if done[i] {
						clog--
						done[i] = false
					}
				}
			}
		}
	}
}

// eat processes a single job. It returns true if the job is completed.
func (tab *table) apply(job *work) bool {
	tab.assert(job.in)
	switch xyz.ValueOf(job.op) {
	case ops.Search:
		return tab.search(job)
	case ops.Output:
		return tab.output(job)
	case ops.Delete:
		return tab.delete(job)
	case ops.Insert:
		return tab.insert(job)
	case ops.Update:
		return tab.update(job)
	default:
		panic(fmt.Sprintf("unexpected type %v", xyz.ValueOf(job.op)))
	}
}

func (col *columns[T]) assert(schema sodium.Column, index bool, size int) {
	if _, ok := col.names[schema.Name]; ok {
		return
	}
	col.names[schema.Name] = len(col.slice)
	col.slice = append(col.slice, column[T]{
		index: make(map[T][]int),
		value: make([]T, size),
	})
}

func (col *columns[T]) malloc(size int) {
	for i := range col.slice {
		col.slice[i].value = append(col.slice[i].value, make([]T, size)...)
	}
}

func (tab *table) assert(table sodium.Table) {
	assert := func(schema sodium.Column, index bool) {
		switch schema.Type {
		case sodium.Values.Bool:
			tab.bool.assert(schema, index, tab.next)
		case sodium.Values.Int8:
			tab.char.assert(schema, index, tab.next)
		case sodium.Values.Int16:
			tab.i16s.assert(schema, index, tab.next)
		case sodium.Values.Int32:
			tab.i32s.assert(schema, index, tab.next)
		case sodium.Values.Int64, sodium.Values.Time:
			tab.i64s.assert(schema, index, tab.next)
		case sodium.Values.Uint8:
			tab.byte.assert(schema, index, tab.next)
		case sodium.Values.Uint16:
			tab.u16s.assert(schema, index, tab.next)
		case sodium.Values.Uint32:
			tab.u32s.assert(schema, index, tab.next)
		case sodium.Values.Uint64:
			tab.u64s.assert(schema, index, tab.next)
		case sodium.Values.Float32:
			tab.f32s.assert(schema, index, tab.next)
		case sodium.Values.Float64:
			tab.f64s.assert(schema, index, tab.next)
		case sodium.Values.String, sodium.Values.Bytes:
			tab.text.assert(schema, index, tab.next)
		}
	}
	for _, schema := range table.Index {
		assert(schema, true)
	}
	for _, schema := range table.Value {
		assert(schema, false)
	}
}

// filter returns true if the specified row matches the given expression.
func (tab *table) filter(row int, expression sodium.Expression) bool {
	compare := func(col sodium.Column, val sodium.Value) int {
		switch xyz.ValueOf(val) {
		case sodium.Values.Bool:
			var u8 uint8
			if sodium.Values.Bool.Get(val) {
				u8 = 1
			}
			return tab.bool.slice[tab.char.names[col.Name]].compare(row, u8)
		case sodium.Values.Int8:
			return tab.char.slice[tab.char.names[col.Name]].compare(row, sodium.Values.Int8.Get(val))
		case sodium.Values.Int16:
			return tab.i16s.slice[tab.i16s.names[col.Name]].compare(row, sodium.Values.Int16.Get(val))
		case sodium.Values.Int32:
			return tab.i32s.slice[tab.i32s.names[col.Name]].compare(row, sodium.Values.Int32.Get(val))
		case sodium.Values.Int64:
			return tab.i64s.slice[tab.i64s.names[col.Name]].compare(row, sodium.Values.Int64.Get(val))
		case sodium.Values.Uint8:
			return tab.byte.slice[tab.byte.names[col.Name]].compare(row, sodium.Values.Uint8.Get(val))
		case sodium.Values.Uint16:
			return tab.u16s.slice[tab.u16s.names[col.Name]].compare(row, sodium.Values.Uint16.Get(val))
		case sodium.Values.Uint32:
			return tab.u32s.slice[tab.u32s.names[col.Name]].compare(row, sodium.Values.Uint32.Get(val))
		case sodium.Values.Uint64:
			return tab.u64s.slice[tab.u64s.names[col.Name]].compare(row, sodium.Values.Uint64.Get(val))
		case sodium.Values.Float32:
			return tab.f32s.slice[tab.f32s.names[col.Name]].compare(row, sodium.Values.Float32.Get(val))
		case sodium.Values.Float64:
			return tab.f64s.slice[tab.f64s.names[col.Name]].compare(row, sodium.Values.Float64.Get(val))
		case sodium.Values.String:
			return tab.text.slice[tab.text.names[col.Name]].compare(row, sodium.Values.String.Get(val))
		case sodium.Values.Bytes:
			return tab.text.slice[tab.text.names[col.Name]].compare(row, string(sodium.Values.Bytes.Get(val)))
		case sodium.Values.Time:
			return tab.i64s.slice[tab.i64s.names[col.Name]].compare(row, sodium.Values.Time.Get(val).UnixNano())
		}
		panic(fmt.Sprintf("unexpected type %v", col.Type))
	}
	empty := func(col sodium.Column) bool {
		switch col.Type {
		case sodium.Values.Bool:
			return tab.bool.slice[tab.bool.names[col.Name]].empty(row)
		case sodium.Values.Int8:
			return tab.char.slice[tab.char.names[col.Name]].empty(row)
		case sodium.Values.Int16:
			return tab.i16s.slice[tab.i16s.names[col.Name]].empty(row)
		case sodium.Values.Int32:
			return tab.i32s.slice[tab.i32s.names[col.Name]].empty(row)
		case sodium.Values.Int64, sodium.Values.Time:
			return tab.i64s.slice[tab.i64s.names[col.Name]].empty(row)
		case sodium.Values.Uint8:
			return tab.byte.slice[tab.byte.names[col.Name]].empty(row)
		case sodium.Values.Uint16:
			return tab.u16s.slice[tab.u16s.names[col.Name]].empty(row)
		case sodium.Values.Uint32:
			return tab.u32s.slice[tab.u32s.names[col.Name]].empty(row)
		case sodium.Values.Uint64:
			return tab.u64s.slice[tab.u64s.names[col.Name]].empty(row)
		case sodium.Values.Float32:
			return tab.f32s.slice[tab.f32s.names[col.Name]].empty(row)
		case sodium.Values.Float64:
			return tab.f64s.slice[tab.f64s.names[col.Name]].empty(row)
		case sodium.Values.String, sodium.Values.Bytes:
			return tab.text.slice[tab.text.names[col.Name]].empty(row)
		}
		panic(fmt.Sprintf("unexpected type %v", col.Type))
	}
	switch xyz.ValueOf(expression) {
	case sodium.Expressions.Index:
		column, index := sodium.Expressions.Index.Get(expression).Split()
		return compare(column, index) == 0
	case sodium.Expressions.Where:
		where := sodium.Expressions.Where.Get(expression)
		switch xyz.ValueOf(where) {
		case sodium.WhereExpressions.LessThan:
			column, less := sodium.WhereExpressions.LessThan.Get(where).Split()
			return compare(column, less) < 0
		case sodium.WhereExpressions.MoreThan:
			column, more := sodium.WhereExpressions.MoreThan.Get(where).Split()
			return compare(column, more) > 0
		case sodium.WhereExpressions.Min:
			column, min := sodium.WhereExpressions.Min.Get(where).Split()
			return compare(column, min) >= 0
		case sodium.WhereExpressions.Max:
			column, max := sodium.WhereExpressions.Max.Get(where).Split()
			return compare(column, max) <= 0
		default:
			panic(fmt.Sprintf("unexpected type %v", xyz.ValueOf(where)))
		}
	case sodium.Expressions.Match:
		match := sodium.Expressions.Match.Get(expression)
		switch xyz.ValueOf(match) {
		case sodium.MatchExpressions.Contains:
			column, contains := sodium.MatchExpressions.Contains.Get(match).Split()
			stored := tab.text.slice[tab.text.names[column.Name]].value[row]
			return strings.Contains(stored, contains)
		case sodium.MatchExpressions.HasPrefix:
			column, prefix := sodium.MatchExpressions.HasPrefix.Get(match).Split()
			stored := tab.text.slice[tab.text.names[column.Name]].value[row]
			return strings.HasPrefix(stored, prefix)
		case sodium.MatchExpressions.HasSuffix:
			column, suffix := sodium.MatchExpressions.HasSuffix.Get(match).Split()
			stored := tab.text.slice[tab.text.names[column.Name]].value[row]
			return strings.HasSuffix(stored, suffix)
		default:
			panic(fmt.Sprintf("unexpected type %v", xyz.ValueOf(match)))
		}
	case sodium.Expressions.Empty:
		column := sodium.Expressions.Empty.Get(expression)
		return empty(column)
	case sodium.Expressions.Avoid:
		avoid := sodium.Expressions.Avoid.Get(expression)
		return !tab.filter(row, avoid)
	case sodium.Expressions.Cases:
		cases := sodium.Expressions.Cases.Get(expression)
		match := false
		for _, expression := range cases {
			if tab.filter(row, expression) {
				match = true
				break
			}
		}
		return match
	case sodium.Expressions.Group:
		group := sodium.Expressions.Group.Get(expression)
		match := true
		for _, expression := range group {
			if !tab.filter(row, expression) {
				match = false
				break
			}
		}
		return match
	default:
		return true
	}
}

func (tab *table) search(job *work) bool {
	if job.at >= tab.next { // move to the next partition?
		job.at = 0
		return true
	}
	var (
		query = ops.Search.Get(job.op)
	)
	for _, expression := range query { // does the current row match the query?
		if !tab.filter(job.at, expression) {
			job.at++ // move job to the next row.
			return false
		}
	}
	read := func(column sodium.Column) sodium.Value {
		switch column.Type {
		case sodium.Values.Bool:
			u8 := tab.bool.slice[tab.bool.names[column.Name]].value[job.at]
			if u8 == 1 {
				return sodium.Values.Bool.As(true)
			} else {
				return sodium.Values.Bool.As(false)
			}
		case sodium.Values.Int8:
			return sodium.Values.Int8.As(tab.char.slice[tab.char.names[column.Name]].value[job.at])
		case sodium.Values.Int16:
			return sodium.Values.Int16.As(tab.i16s.slice[tab.i16s.names[column.Name]].value[job.at])
		case sodium.Values.Int32:
			return sodium.Values.Int32.As(tab.i32s.slice[tab.i32s.names[column.Name]].value[job.at])
		case sodium.Values.Int64:
			return sodium.Values.Int64.As(tab.i64s.slice[tab.i64s.names[column.Name]].value[job.at])
		case sodium.Values.Uint8:
			return sodium.Values.Uint8.As(tab.byte.slice[tab.byte.names[column.Name]].value[job.at])
		case sodium.Values.Uint16:
			return sodium.Values.Uint16.As(tab.u16s.slice[tab.u16s.names[column.Name]].value[job.at])
		case sodium.Values.Uint32:
			return sodium.Values.Uint32.As(tab.u32s.slice[tab.u32s.names[column.Name]].value[job.at])
		case sodium.Values.Uint64:
			return sodium.Values.Uint64.As(tab.u64s.slice[tab.u64s.names[column.Name]].value[job.at])
		case sodium.Values.Float32:
			return sodium.Values.Float32.As(tab.f32s.slice[tab.f32s.names[column.Name]].value[job.at])
		case sodium.Values.Float64:
			return sodium.Values.Float64.As(tab.f64s.slice[tab.f64s.names[column.Name]].value[job.at])
		case sodium.Values.String:
			return sodium.Values.String.As(tab.text.slice[tab.text.names[column.Name]].value[job.at])
		case sodium.Values.Bytes:
			return sodium.Values.Bytes.As([]byte(tab.text.slice[tab.text.names[column.Name]].value[job.at]))
		case sodium.Values.Time:
			return sodium.Values.Time.As(time.Unix(0, tab.i64s.slice[tab.i64s.names[column.Name]].value[job.at]))
		default:
			panic(fmt.Sprintf("unexpected type %v", column.Type))
		}
	}
	var (
		values = make([]sodium.Value, len(job.in.Index)+len(job.in.Value))
	)
	for i, column := range job.in.Index {
		values[i] = read(column)
	}
	for i, column := range job.in.Value {
		values[i+len(job.in.Index)] = read(column)
	}
	select {
	case job.ch <- values:
	default:
		job.er = fmt.Errorf("search result channel full")
		return true
	}
	job.at++
	return false
}

func (*table) output(job *work) bool {
	job.er = errors.New("not implemented")
	return true
}

func (tab *table) delete(job *work) bool {
	if job.at >= tab.next { // move to the next partition?
		job.at = 0
		return true
	}
	var (
		query = ops.Delete.Get(job.op)
	)
	for _, expression := range query { // does the current row match the query?
		if !tab.filter(job.at, expression) {
			job.at++ // move job to the next row.
			return false
		}
	}
	for i := range tab.bool.slice {
		tab.bool.slice[i].delete(job.at)
	}
	for i := range tab.char.slice {
		tab.char.slice[i].delete(job.at)
	}
	for i := range tab.i16s.slice {
		tab.i16s.slice[i].delete(job.at)
	}
	for i := range tab.i32s.slice {
		tab.i32s.slice[i].delete(job.at)
	}
	for i := range tab.i64s.slice {
		tab.i64s.slice[i].delete(job.at)
	}
	for i := range tab.byte.slice {
		tab.byte.slice[i].delete(job.at)
	}
	for i := range tab.u16s.slice {
		tab.u16s.slice[i].delete(job.at)
	}
	for i := range tab.u32s.slice {
		tab.u32s.slice[i].delete(job.at)
	}
	for i := range tab.u64s.slice {
		tab.u64s.slice[i].delete(job.at)
	}
	for i := range tab.f32s.slice {
		tab.f32s.slice[i].delete(job.at)
	}
	for i := range tab.f64s.slice {
		tab.f64s.slice[i].delete(job.at)
	}
	for i := range tab.text.slice {
		tab.text.slice[i].delete(job.at)
	}
	tab.next--
	job.id++
	job.at++
	return false
}

func (tab *table) insert(job *work) bool {
	index, flag, value := ops.Insert.Get(job.op).Split()
	hash, addr, part, ok := tab.keys.lookup(index, tab.cpus)
	if tab.part != part {
		return true
	}
	job.ok = part // we don't need to pass this job along.
	if ok && !flag {
		return true
	}
	if !ok {
		addr = tab.next
		tab.next++
		tab.keys[hash] = append(tab.keys[hash], pkey{
			index: addr,
			value: index,
		})
		tab.char.malloc(1)
		tab.i16s.malloc(1)
		tab.i32s.malloc(1)
		tab.i64s.malloc(1)
		tab.byte.malloc(1)
		tab.u16s.malloc(1)
		tab.u32s.malloc(1)
		tab.u64s.malloc(1)
		tab.f32s.malloc(1)
		tab.f64s.malloc(1)
		tab.text.malloc(1)
	}
	for i, val := range index {
		tab.write(addr, job.in.Index[i].Name, val)
	}
	for i, val := range value {
		tab.write(addr, job.in.Value[i].Name, val)
	}
	job.id = addr
	return true
}

func (tab *table) update(job *work) bool {
	if job.at >= tab.next { // move to the next partition?
		job.at = 0
		return true
	}
	var (
		query, patch = ops.Update.Get(job.op).Split()
	)
	for _, expression := range query { // does the current row match the query?
		if !tab.filter(job.at, expression) {
			job.at++ // move job to the next row.
			return false
		}
	}
	var modify func(mod sodium.Modification) error
	modify = func(mod sodium.Modification) error {
		switch xyz.ValueOf(mod) {
		case sodium.Modifications.Set:
			column, value := sodium.Modifications.Set.Get(mod).Split()
			tab.write(job.at, column.Name, value)
		case sodium.Modifications.Arr:
			modifications := sodium.Modifications.Arr.Get(mod)
			for _, modification := range modifications {
				if err := modify(modification); err != nil {
					return xray.Error(err)
				}
			}
		default:
			return fmt.Errorf("unsupported modification %v", xyz.ValueOf(mod))
		}
		return nil
	}
	for _, modification := range patch {
		if err := modify(modification); err != nil {
			job.er = err
			return true
		}
	}
	job.id++
	job.at++
	return false
}

// write the given value into the named column at the given column index address.
func (tab *table) write(addr int, name string, val sodium.Value) {
	switch xyz.ValueOf(val) {
	case sodium.Values.Bool:
		col := tab.bool.names[name]
		if sodium.Values.Bool.Get(val) {
			tab.bool.slice[col].value[addr] = 1
		} else {
			tab.bool.slice[col].value[addr] = 0
		}
	case sodium.Values.Int8:
		col := tab.char.names[name]
		tab.char.slice[col].value[addr] = sodium.Values.Int8.Get(val)
	case sodium.Values.Int16:
		col := tab.i16s.names[name]
		tab.i16s.slice[col].value[addr] = sodium.Values.Int16.Get(val)
	case sodium.Values.Int32:
		col := tab.i32s.names[name]
		tab.i32s.slice[col].value[addr] = sodium.Values.Int32.Get(val)
	case sodium.Values.Int64:
		col := tab.i64s.names[name]
		tab.i64s.slice[col].value[addr] = sodium.Values.Int64.Get(val)
	case sodium.Values.Uint8:
		col := tab.byte.names[name]
		tab.byte.slice[col].value[addr] = sodium.Values.Uint8.Get(val)
	case sodium.Values.Uint16:
		col := tab.u16s.names[name]
		tab.u16s.slice[col].value[addr] = sodium.Values.Uint16.Get(val)
	case sodium.Values.Uint32:
		col := tab.u32s.names[name]
		tab.u32s.slice[col].value[addr] = sodium.Values.Uint32.Get(val)
	case sodium.Values.Uint64:
		col := tab.u64s.names[name]
		tab.u64s.slice[col].value[addr] = sodium.Values.Uint64.Get(val)
	case sodium.Values.Time:
		col := tab.i64s.names[name]
		tab.i64s.slice[col].value[addr] = sodium.Values.Time.Get(val).UnixNano()
	case sodium.Values.Float32:
		col := tab.f32s.names[name]
		tab.f32s.slice[col].value[addr] = sodium.Values.Float32.Get(val)
	case sodium.Values.Float64:
		col := tab.f64s.names[name]
		tab.f64s.slice[col].value[addr] = sodium.Values.Float64.Get(val)
	case sodium.Values.String:
		col := tab.text.names[name]
		tab.text.slice[col].value[addr] = sodium.Values.String.Get(val)
	case sodium.Values.Bytes:
		col := tab.text.names[name]
		tab.text.slice[col].value[addr] = string(sodium.Values.Bytes.Get(val))
	}
}
