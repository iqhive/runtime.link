const std = @import("std");
const stdout = std.io.getStdOut().writer();

pub const rkind = enum { Invalid, Bool, Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Uintptr, Float32, Float64, Complex64, Complex128, Array, Chan, Func, Interface, Map, Pointer, Slice, String, Struct, UnsafePointer };

pub const rtype = struct {
    name: []const u8,
    kind: rkind,
    data: rdata,
};

pub const @"int.(type)" = rtype{
    .name = "int",
    .kind = rkind.Int,
    .data = rdata{.Int = void{}},
};

pub const interface = struct {
    rtype: ?*const rtype,
    value: ?*anyopaque,

    pub fn pack(go: *G, comptime T: type, vtype: *const rtype, value: T) interface {
        const ptr = go.new(T);
        ptr.* = value;
        return interface{
            .rtype = vtype,
            .value = ptr,
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
    Array: isize,
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
    name: []const u8,
    type: *const rtype,
    offset: usize,
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
    return std.ArrayListUnmanaged(T);
}

pub fn map(comptime K: type, comptime V: type) type {
    return struct {
        hashmap: *std.AutoHashMapUnmanaged(K, V),

        pub fn set(self: map(K, V), go: *G, key: K, value: V) void {
            self.hashmap.put(go.memory.allocator(), key, value) catch |err| @panic(@errorName(err));
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

        pub fn set(self: smap(V), go: *G, key: []const u8, value: V) void {
            self.hashmap.put(go.memory.allocator(), key, value) catch |err| @panic(@errorName(err));
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

pub const G = struct {
    memory: std.heap.ArenaAllocator = std.heap.ArenaAllocator.init(std.heap.page_allocator),

    pub fn use(_: *G, _: anytype) void {}
    pub fn new(go: *G, T: type) *T {
        const ptr = go.memory.allocator().create(T) catch |err| @panic(@errorName(err));
        ptr.* = std.mem.zeroes(T);
        return ptr;
    }
    pub fn makeSlice(go: *G, T: type, len: usize, cap: usize) slice(T) {
        var array = slice(T).initCapacity(go.memory.allocator(), cap) catch |err| @panic(@errorName(err));
        array.resize(go.memory.allocator(), len) catch |err| @panic(@errorName(err));
        return array;
    }
    pub fn make_map(go: *G, K: type, V: type, cap: usize) map(K, V) {
        var val = map(K, V){
            .hashmap = go.new(std.AutoHashMapUnmanaged(K, V)),
        };
        val.hashmap.* = .{};
        if (cap > 0) {
            val.hashmap.ensureTotalCapacity(go.memory.allocator(), @intCast(cap)) catch |err| @panic(@errorName(err));
        }
        return val;
    }
    pub fn make_smap(go: *G, V: type, cap: usize) smap(V) {
        var val = smap(V){
            .hashmap = go.new(std.StringHashMapUnmanaged(V)),
        };
        val.hashmap.* = .{};
        if (cap > 0) {
            val.hashmap.ensureTotalCapacity(go.memory.allocator(), @intCast(cap)) catch |err| @panic(@errorName(err));
        }
        return val;
    }
    pub fn append(go: *G, comptime T: type, array: slice(T), elem: T) slice(T) {
        var clone = array;
        clone.append(go.memory.allocator(), elem) catch |err| @panic(@errorName(err));
        return clone;
    }
    pub fn copy(_: *G, comptime T: type, dst: slice(T), src: slice(T)) isize {
        std.mem.copyForwards(T, dst.items, src.items);
        return @intCast(@min(dst.items.len, src.items.len));
    }
    pub fn exit(go: *G) void {
        go.memory.deinit();
    }
    pub fn println(_: *G, comptime fmt: []const u8, args: anytype) void {
        std.debug.print(fmt, args);
        std.debug.print("\n", .{});
    }

    pub fn rptr(go: *G,elem: *const rtype) *const rtype {
        const ptr = go.memory.allocator().create(rtype) catch |err| @panic(@errorName(err));
        ptr.* = rtype{
            .name = "",
            .kind = rkind.Pointer,
            .data = rdata{.Pointer = elem},
        };
        return ptr;
    }
};
