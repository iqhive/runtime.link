# ZGO

A data-race free safer subset of the Go specification + optimisations.

Additional Rules:
1. Global variables cannot be mutated after init.
2. Initialisation must be deterministic (no system calls, no range-over-map, no IO).
3. Reference values passed via channels, or captured by goroutines, must remain immutable.
4. All started goroutines must immediately call recover in a deferred function, this is the only valid scenario for recover.
5. With the exception of immediately ranging over the channel, any potentially cross-goroutine receivers of a channel must select with either a default, or `context.Done` case.
6. The Index field on `reflect.StructField` is immutable.

Optimisation Goals:
1. No garbage collector, each goroutine gets a memory arena for allocations that is freed on exit (although may eventually have a goroutine-local GC) .
2. Receive-only channels with single producers are much faster and suitable for use as iterators and to coordinate 'coroutines'.
3. Functions that only return values, ie. `func() (T...)` are represented as tuples in memory without allocation in the case that they simply return their captured values.
4. Zero-overhead C calls, CGO handles keepalive the  goroutine that allocated them with reference counting.
5. Channels are reference counted per referencing goroutine, goroutines waiting to send or receive on the only remaining reference to a channel will automatically exit.
6. Entire program can be compiled as a distributed system such that goroutines can be executed on different machines, separated over the network.
7. Wider interface values in memory that can store up to 128bit values without allocating.
8. Faster reflection with less allocations.
9. Smaller binary size (maybe).