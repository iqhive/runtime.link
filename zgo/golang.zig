const std = @import("std");

// basic types.
pub const int = isize;
pub const int8 = i8;
pub const int16 = i16;
pub const int32 = i32;
pub const int64 = i64;
pub const uint = usize;
pub const uint8 = u8;
pub const uint16 = u16;
pub const uint32 = u32;
pub const uint64 = u64;
pub const uintptr = usize;
pub const float32 = f32;
pub const float64 = f64;
pub const complex64 = std.complex.Complex(f32);
pub const complex128 = std.complex.Complex(f64);
pub const rune = i32;
pub const byte = u8;
pub const string = []const byte;

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
    data: rdata,
};

pub const @"int.(type)" = rtype{
    .name = "int",
    .kind = rkind.Int,
    .data = rdata{ .Int = void{} },
};

pub const interface = struct {
    rtype: ?*const rtype,
    value: ?*anyopaque,

    pub fn pack(comptime T: type, vtype: *const rtype, value: T) interface {
        const p = runtime.memory.allocator().create(T) catch |err| @panic(@errorName(err));
        p.* = value;
        return interface{
            .rtype = vtype,
            .value = p,
        };
    }
};

pub const rdata = union(rkind) {
    Invalid: void,
    Bool: void,
    Int: void,
    Int8: void,
    Int16: void,
    Int32: void,
    Int64: void,
    Uint: void,
    Uint8: void,
    Uint16: void,
    Uint32: void,
    Uint64: void,
    Uintptr: void,
    Float32: void,
    Float64: void,
    Complex64: void,
    Complex128: void,
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

pub const field = struct {
    name: string,
    type: *const rtype,
    offset: uintptr,
    exported: bool,
    embedded: bool,
};

pub const rchan = struct {
    elem: *const rtype,
    send: bool,
    recv: bool,
};

pub const rfunc = struct {
    name: []const u8,
    vary: bool,
    args: []*const rtype,
    rets: []*const rtype,
};

pub fn slice(comptime T: type) type {
    return struct {
        arraylist: std.ArrayListUnmanaged(T),

        pub fn make(len: usize, cap: usize) slice(T) {
            var array = std.ArrayListUnmanaged(T).initCapacity(runtime.memory.allocator(), cap) catch |err| @panic(@errorName(err));
            array.resize(runtime.memory.allocator(), len) catch |err| @panic(@errorName(err));
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

pub fn map(comptime K: type, comptime V: type) type {
    return struct {
        hashmap: *std.AutoHashMapUnmanaged(K, V),

        pub fn make(cap: int) map(K, V) {
            var val = map(K, V){
                .hashmap = runtime.memory.allocator().create(std.AutoHashMapUnmanaged(V)) catch |err| @panic(@errorName(err)),
            };
            val.hashmap.* = .{};
            if (cap > 0) {
                val.hashmap.ensureTotalCapacity(runtime.memory.allocator(), @intCast(cap)) catch |err| @panic(@errorName(err));
            }
            return val;
        }
        pub fn set(self: map(K, V), key: K, value: V) void {
            self.hashmap.put(runtime.memory.allocator(), key, value) catch |err| @panic(@errorName(err));
        }
        pub fn get(self: map(K, V), key: K) V {
            if (self.hashmap.get(key)) |val| {
                return val;
            }
            return std.mem.zeroes(V);
        }
        pub fn clear(self: map(K, V), go: *G) void {
            self.hashmap.clearRetainingCapacity(go.memory.allocator());
        }
    };
}

pub fn smap(comptime V: type) type {
    return struct {
        hashmap: *std.StringHashMapUnmanaged(V),

        pub fn make(cap: int) smap(V) {
            var val = smap(V){
                .hashmap = runtime.memory.allocator().create(std.StringHashMapUnmanaged(V)) catch |err| @panic(@errorName(err)),
            };
            val.hashmap.* = .{};
            if (cap > 0) {
                val.hashmap.ensureTotalCapacity(runtime.memory.allocator(), @intCast(cap)) catch |err| @panic(@errorName(err));
            }
            return val;
        }
        pub fn set(self: smap(V), key: []const u8, value: V) void {
            self.hashmap.put(runtime.memory.allocator(), key, value) catch |err| @panic(@errorName(err));
        }
        pub fn get(self: smap(V), key: []const u8) V {
            if (self.hashmap.get(key)) |val| {
                return val;
            }
            return std.mem.zeroes(V);
        }
        pub fn clear(self: smap(V), go: *G) void {
            self.hashmap.clearRetainingCapacity(go.memory.allocator());
        }
    };
}

pub fn types(comptime list: []const type) type {
    return std.meta.Tuple(list);
}

pub fn func(comptime I: type, comptime O: type) type {
    return struct {
        closure: ?*const anyopaque,
        pointer: ?*const fn(ctx: *const anyopaque, args: I) O,

        fn call(self: func(I, O), args: I) O {
            if (!self.pointer) {
                @panic("nil function call");
            }
            return self.pointer(self.closure, args);
        }
    };
}

pub threadlocal var runtime: G = G{};

pub const G = struct {
    memory: std.heap.ArenaAllocator = std.heap.ArenaAllocator.init(std.heap.page_allocator),
};

pub fn new(T: type) pointer(T) {
    const p = runtime.memory.allocator().create(T) catch |err| @panic(@errorName(err));
    p.* = std.mem.zeroes(T);
    return pointer(T){ .address = p };
}

pub fn append(comptime T: type, array: slice(T), elem: T) slice(T) {
    var clone = array;
    clone.arraylist.append(runtime.memory.allocator(), elem) catch |err| @panic(@errorName(err));
    return clone;
}
pub fn copy(comptime T: type, dst: slice(T), src: slice(T)) int {
    std.mem.copyForwards(T, dst.arraylist.items, src.arraylist.items);
    return @intCast(@min(dst.arraylist.items.len, src.arraylist.items.len));
}
pub fn exit() void {
    runtime.memory.deinit();
}
pub fn println(comptime fmt: []const u8, args: anytype) void {
    std.debug.print(fmt, args);
    std.debug.print("\n", .{});
}

pub fn rptr(elem: *const rtype) *const rtype {
    const p = runtime.memory.allocator().create(rtype) catch |err| @panic(@errorName(err));
    p.* = rtype{
        .name = "",
        .kind = rkind.Pointer,
        .data = rdata{ .Pointer = elem },
    };
    return p;
}
