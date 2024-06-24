const std = @import("std");

// builtin types.
pub const int = isize;
pub const @"int.(type)" = rtype.make("int", rkind.Int);
pub const int8 = i8;
pub const @"int8.(type)" = rtype.make("int8", rkind.Int8);
pub const int16 = i16;
pub const @"int16.(type)" = rtype.make("int16", rkind.Int16);
pub const int32 = i32;
pub const @"int32.(type)" = rtype.make("int32", rkind.Int32);
pub const int64 = i64;
pub const @"int64.(type)" = rtype.make("int64", rkind.Int64);
pub const uint = usize;
pub const @"uint.(type)" = rtype.make("uint", rkind.Uint);
pub const uint8 = u8;
pub const @"uint8.(type)" = rtype.make("uint8", rkind.Uint8);
pub const uint16 = u16;
pub const @"uint16.(type)" = rtype.make("uint16", rkind.Uint16);
pub const uint32 = u32;
pub const @"uint32.(type)" = rtype.make("uint32", rkind.Uint32);
pub const uint64 = u64;
pub const @"uint64.(type)" = rtype.make("uint64", rkind.Uint64);
pub const uintptr = usize;
pub const @"uintptr.(type)" = rtype.make("uintptr", rkind.Uintptr);
pub const float32 = f32;
pub const @"float32.(type)" = rtype.make("float32", rkind.Float32);
pub const float64 = f64;
pub const @"float64.(type)" = rtype.make("float64", rkind.Float64);
pub const complex64 = std.complex.Complex(f32);
pub const @"complex64.(type)" = rtype.make("complex64", rkind.Complex64);
pub const complex128 = std.complex.Complex(f64);
pub const @"complex128.(type)" = rtype.make("complex128", rkind.Complex128);
pub const string = []const byte;
pub const @"string.(type)" = rtype.make("string", rkind.String);
pub const @"error" = interface(struct { Error: fn (*anyopaque, *routine) string });
pub const @"error.(type)" = rtype{
    .name = "error",
    .kind = rkind.Interface,
    .data = rdata{ .Interface = []rfunc{.{
        .name = "Error",
        .vary = false,
        .call = null,
        .wrap = null,
        .args = []*rtype{},
        .rets = []*rtype{@"string.(type)"},
    }} },
};

// builtin aliases.
pub const rune = i32;
pub const byte = u8;

// rkind represents the reflect.Kind type.
pub const rkind = enum { Invalid, Bool, Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Uintptr, Float32, Float64, Complex64, Complex128, Array, Chan, Func, Interface, Map, Pointer, Slice, String, Struct, UnsafePointer };

// use can be used to bypass unused variable/arguments errors.
pub fn use(_: anytype) void {}

// zero returns the zero value for a Go type.
pub fn zero(comptime T: type) T {
    return std.mem.zeroes(T);
}

// rtype is the reflection structure for a Go type.
pub const rtype = struct {
    name: string,
    kind: rkind,
    data: rdata = rdata{ .Void = void{} },

    pub fn make(name: string, kind: rkind) rtype {
        return rtype{
            .name = name,
            .kind = kind,
        };
    }
};

// any represents the Go empty interface type.
pub const any = struct {
    rtype: ?*const rtype,
    value: ?*anyopaque,

    pub fn make(comptime T: type, goto: *routine, vtype: *const rtype, value: T) any {
        const p = goto.memory.allocator().create(T) catch |err| @panic(@errorName(err));
        p.* = value;
        return any{
            .rtype = vtype,
            .value = p,
        };
    }
};

// rdata contains type-specific data for a Go type.
pub const rdata = union {
    Void: void,
    Array: int,
    Chan: rchan,
    Func: rfunc,
    Interface: []rfunc,
    Map: [2]*const rtype,
    Pointer: *const rtype,
    Slice: *const rtype,
    String: void,
    Struct: []field,
    UnsafePointer: void,
};

// field used for reflection.
pub const field = struct {
    name: string,
    type: *const rtype,
    offset: uintptr,
    exported: bool,
    embedded: bool,
};

// rchan represents the reflection of a
// channel and its direction.
pub const rchan = struct {
    elem: *const rtype,
    send: bool,
    recv: bool,
};

// rfunc represents the reflection of a
// Go function.
pub const rfunc = struct {
    name: string,
    vary: bool,
    call: *anyopaque,
    wrap: *const fn (*routine, []any, []any) void,
    args: []*const rtype,
    rets: []*const rtype,
};

// slice represents a Go slice.
pub fn slice(comptime T: type) type {
    return struct {
        arraylist: std.ArrayListUnmanaged(T),

        pub fn make(goto: *routine, len: usize, cap: usize) slice(T) {
            var array = std.ArrayListUnmanaged(T).initCapacity(goto.memory.allocator(), cap) catch |err| @panic(@errorName(err));
            array.resize(goto.memory.allocator(), len) catch |err| @panic(@errorName(err));
            return slice(T){
                .arraylist = array,
            };
        }
        pub fn index(self: slice(T), i: int) T {
            if (i < 0 or i >= self.arraylist.items.len) {
                @panic("index out of range");
            }
            return self.arraylist.items[@intCast(i)];
        }
        pub fn clear(self: slice(T)) void {
            @memset(self.arraylist.items, zero(T));
        }
        pub fn range(self: slice(T), pos: int, end: int) slice(T) {
            if (pos < 0 or pos > self.arraylist.items.len or end < 0 or end > self.arraylist.items.len) {
                @panic("slice index out of range");
            }
            return slice(T){
                .arraylist = self.arraylist.slice(pos, end),
            };
        }
    };
}

// pointer represents a Go pointer.
pub fn pointer(comptime T: type) type {
    return struct {
        address: ?*T,

        pub fn set(self: pointer(T), val: T) void {
            if (self.address) |p| {
                p.* = val;
            } else {
                @panic("nil pointer dereference");
            }
        }
        pub fn get(self: pointer(T)) T {
            if (self.address) |p| {
                return p.*;
            } else {
                @panic("nil pointer dereference");
            }
        }
        pub fn range(self: pointer(T), pos: int, end: int) slice(@typeInfo(T).Array.child) {
            if (pos < 0 or pos > end or end > @typeInfo(T).Array.len) {
                @panic("slice index out of range");
            }
            if (self.address) |a| {
                var result = slice(@typeInfo(T).Array.child){
                    .arraylist = std.ArrayListUnmanaged(@typeInfo(T).Array.child){},
                };
                result.arraylist.items = a.*[@intCast(pos)..@intCast(end)];
                return result;
            } else {
                @panic("nil pointer dereference");
            }
        }
    };
}

pub fn interface(comptime T: type) type {
    return struct {
        rtype: *const rtype,
        itype: *T,
        value: *anyopaque,
    };
}

// map represents a Go map.
pub fn map(comptime K: type, comptime V: type) type {
    return struct {
        hashmap: *std.AutoHashMapUnmanaged(K, V),

        pub fn make(goto: *routine, cap: int) map(K, V) {
            var val = map(K, V){
                .hashmap = goto.memory.allocator().create(std.AutoHashMapUnmanaged(V)) catch |err| @panic(@errorName(err)),
            };
            val.hashmap.* = .{};
            if (cap > 0) {
                val.hashmap.ensureTotalCapacity(goto.memory.allocator(), @intCast(cap)) catch |err| @panic(@errorName(err));
            }
            return val;
        }
        pub fn set(self: map(K, V), goto: *routine, key: K, value: V) void {
            self.hashmap.put(goto.memory.allocator(), key, value) catch |err| @panic(@errorName(err));
        }
        pub fn get(self: map(K, V), key: K) V {
            if (self.hashmap.get(key)) |val| {
                return val;
            }
            return std.mem.zeroes(V);
        }
        pub fn clear(self: map(K, V), goto: *routine) void {
            self.hashmap.clearRetainingCapacity(goto.memory.allocator());
        }
    };
}

// smap represents a Go map with string keys.
pub fn smap(comptime V: type) type {
    return struct {
        hashmap: *std.StringHashMapUnmanaged(V),

        pub fn make(goto: *routine, cap: int) smap(V) {
            var val = smap(V){
                .hashmap = goto.memory.allocator().create(std.StringHashMapUnmanaged(V)) catch |err| @panic(@errorName(err)),
            };
            val.hashmap.* = .{};
            if (cap > 0) {
                val.hashmap.ensureTotalCapacity(goto.memory.allocator(), @intCast(cap)) catch |err| @panic(@errorName(err));
            }
            return val;
        }
        pub fn set(self: smap(V), goto: *routine, key: string, value: V) void {
            self.hashmap.put(goto.memory.allocator(), key, value) catch |err| @panic(@errorName(err));
        }
        pub fn get(self: smap(V), key: string) V {
            if (self.hashmap.get(key)) |val| {
                return val;
            }
            return std.mem.zeroes(V);
        }
        pub fn clear(self: smap(V), goto: *routine) void {
            self.hashmap.clearRetainingCapacity(goto.memory.allocator());
        }
    };
}

// types used when defining function types.
pub fn types(comptime list: []const type) type {
    return std.meta.Tuple(list);
}

pub fn signature(comptime T: ?type) type {
    if (T) |vtype| {
        return vtype;
    }
    return void;
}

// func is a Go function type.
pub fn func(comptime T: type) type {
    return struct {
        closure: ?*const anyopaque,
        pointer: ?*const T,
        wrapper: ?*const fn (ctx: *const anyopaque, goto: *routine, args: []any, rets: []any) void = null,

        pub fn make(V: anytype) func(T) {
            const parent = @typeInfo(@TypeOf(V)).Pointer.child;
            return func(T){
                .closure = @ptrCast(V),
                .pointer = @field(parent, "call"),
                //.wrapper = @field(parent, "wrap"),
            };
        }
        pub fn call(self: func(T), args: anytype) signature(@typeInfo(T).Fn.return_type) {
            if (self.pointer) |f| {
                if (self.closure) |c| {
                    return @call(.auto, f, .{c} ++ args);
                }
            }
            @panic("nil function pointer dereference");
        }
        pub fn go(self: func(T), args: anytype) void {
            use(std.Thread.spawn(.{}, call, .{ self, args }));
        }
    };
}

// chan represents a Go channel.
pub fn chan(comptime T: type) type {
    const ring = struct {
        buf: std.RingBuffer,
        mut: std.Thread.Mutex = .{},
        sig: std.Thread.Condition = .{},
        bad: bool = false,
        sub: uint = 0,
    };
    return struct {
        pointer: ?*ring,

        pub fn make(goto: *routine, cap: int) chan(T) {
            const val = chan(T){
                .pointer = goto.memory.allocator().create(ring) catch |err| @panic(@errorName(err)),
            };
            if (val.pointer) |p| {
                p.* = ring{
                    .buf = std.RingBuffer.init(goto.memory.allocator(), @as(usize, @intCast(cap))*@sizeOf(T)) catch |err| @panic(@errorName(err)),
                };
            }
            return val;
        }
        pub fn send(self: chan(T), goto: *routine, value: T) void {
            while(true) {
                if (self.pointer) |r| {
                    r.mut.lock();
                    defer r.mut.unlock();
                    if (r.bad) @panic("send on closed channel");
                    r.buf.writeSlice(std.mem.asBytes(&value)) catch continue;
                    if (r.sub > 0) {
                        r.sig.signal();
                    }
                } else {
                    goto.exit();
                }
                return;
            }
        }
        pub fn recv(self: chan(T), goto: *routine) T {
            var value = zero(T);
            if (self.pointer) |r| {
                r.mut.lock();
                defer r.mut.unlock();
                if (r.bad) return value;
                r.sub += 1;
                if (r.buf.isEmpty()) {
                    r.sig.wait(&r.mut);
                }
                r.sub -= 1;
                const bytes = std.mem.asBytes(&value);
                r.buf.readFirst(bytes, bytes.len) catch |err| @panic(@errorName(err));
            } else {
                goto.exit();
            }
            return value;
        }
    };
}

// routine represents the state for a goroutine.
pub const routine = struct {
    memory: std.heap.ArenaAllocator = std.heap.ArenaAllocator.init(std.heap.page_allocator),

    pub fn exit(goto: *routine) void {
        goto.memory.deinit();
    }
};

pub fn new(goto: *routine, T: type) pointer(T) {
    const p = goto.memory.allocator().create(T) catch |err| @panic(@errorName(err));
    p.* = std.mem.zeroes(T);
    return pointer(T){ .address = p };
}

pub fn append(goto: *routine, comptime T: type, array: slice(T), elem: T) slice(T) {
    var clone = array;
    clone.arraylist.append(goto.memory.allocator(), elem) catch |err| @panic(@errorName(err));
    return clone;
}

pub fn copy(comptime T: type, dst: slice(T), src: slice(T)) int {
    std.mem.copyForwards(T, dst.arraylist.items, src.arraylist.items);
    return @intCast(@min(dst.arraylist.items.len, src.arraylist.items.len));
}

pub fn println(comptime fmt: string, args: anytype) void {
    std.debug.print(fmt, args);
    std.debug.print("\n", .{});
}

// rptr implements reflect.PtrTo.
pub fn rptr(goto: *routine, elem: *const rtype) *const rtype {
    const p = goto.memory.allocator().create(rtype) catch |err| @panic(@errorName(err));
    p.* = rtype{
        .name = "",
        .kind = rkind.Pointer,
        .data = rdata{ .Pointer = elem },
    };
    return p;
}

pub fn go(comptime function: anytype, args: anytype) void {
    std.Thread.spawn(.{}, function, args);
}

pub const testing = struct{
    pub fn FailNow(_: testing, _: *routine) void {
        std.testing.expect(false) catch |err| @panic(@errorName(err));
    }
};