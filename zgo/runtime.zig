const std = @import("std");
const stdout = std.io.getStdOut().writer();

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
};