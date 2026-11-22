// Functions for the tests in this package to call. They are built into a shared
// library by each C compiler that is installed, as gcc and clang disagree about
// what a callee may assume about its arguments.
#include <stdarg.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>

// Reads the string after sleeping, like any C function that holds on to its
// argument across a blocking operation.
int slow_strlen(const char *s, int ms) {
	usleep(ms * 1000);
	return (int)strlen(s);
}

int ident_uchar(unsigned char c) { return c; }
int ident_ushort(unsigned short c) { return c; }
int ident_bool(bool b) { return b ? 11 : 22; }

uintptr_t ptr_ident(void *p) { return (uintptr_t)p; }
char *echo_str(char *p) { return p; }
char *null_str(void) { return NULL; }
char *static_str(void) { return "static"; }
int is_null(void *p) { return p == NULL; }

int call_cb(int (*fn)(int), int v) { return fn(v); }
const char *call_str_cb(int (*fn)(const char *)) { return fn("from C") == 6 ? "ok" : "bad"; }

static int (*kept)(int);
void keep_cb(int (*fn)(int)) { kept = fn; }
uintptr_t keep_cb_ident(int (*fn)(int)) { kept = fn; return (uintptr_t)fn; }
int call_kept(int v) { return kept(v); }

typedef int (*intfn)(int);
static int twice(int v) { return v * 2; }
intfn get_twice(void) { return twice; }
intfn get_null(void) { return NULL; }

double add_double(double (*fn)(double, double), double a, double b) { return fn(a, b); }
void call_with_fn(void (*fn)(int (*)(int))) { fn(twice); }

int first_of(int a, int b) { return a; }
long sub2(long a, long b) { return a - b; }
int add3(int a, int b, int c) { return a + b + c; }
bool is_nonzero(int a) { return a != 0; }

int last_error(void *p) { return p == NULL ? 7 : 9; }

// Returns the last of n stack-passed arguments, past where a fixed
// size argument buffer would run out.
long last_of(int n, ...) {
	va_list args;
	va_start(args, n);
	long last = 0;
	for (int i = 0; i < n; i++) last = va_arg(args, long);
	va_end(args);
	return last;
}

// Tells the libraries built by each compiler apart.
int compiled_by_clang(void) {
#ifdef __clang__
	return 1;
#else
	return 0;
#endif
}

long len_then_str(size_t n, const char *s) { return (long)n * 1000 + (long)strlen(s); }
long str_len_int(const char *s, size_t n, int k) { return (long)strlen(s) * 1000000 + (long)n * 1000 + k; }
long str_len_int_last(size_t n, const char *s, int k) { return (long)strlen(s) * 1000000 + (long)n * 1000 + k; }

// Counts what it was called with, so a test can tell whether it was called.
static int bumps;
void bump(void) { bumps++; }
int bump_by(int n) { bumps += n; return 0; }
int bumped(void) { return bumps; }

// Stand-ins for toupper and tolower that every platform's library has.
int upper(int c) { return c >= 'a' && c <= 'z' ? c - 32 : c; }
int lower(int c) { return c >= 'A' && c <= 'Z' ? c + 32 : c; }

double half(double x) { return x / 2; }
float halff(float x) { return x / 2; }
double first_double(double a, size_t b) { return a; }

// Variadic functions read each of their variable arguments with the type that C
// promotes it to, from wherever the platform's convention puts such arguments.
double sum_doubles(int n, ...) {
	va_list args;
	va_start(args, n);
	double sum = 0;
	for (int i = 0; i < n; i++) sum += va_arg(args, double);
	va_end(args);
	return sum;
}

// The named float is passed as a float, where the variable ones are doubles.
double scale_doubles(float scale, int n, ...) {
	va_list args;
	va_start(args, n);
	double sum = 0;
	for (int i = 0; i < n; i++) sum += va_arg(args, double);
	va_end(args);
	return scale * sum;
}

// Reads n pairs of a long and a double, and adds up the long and 1000 times the double of each.
long sum_pairs(int n, ...) {
	va_list args;
	va_start(args, n);
	long sum = 0;
	for (int i = 0; i < n; i++) {
		sum += va_arg(args, long);
		sum += (long)(va_arg(args, double) * 1000);
	}
	va_end(args);
	return sum;
}

int format_into(char *buf, size_t size, const char *format, ...) {
	va_list args;
	va_start(args, format);
	int n = vsnprintf(buf, size, format, args);
	va_end(args);
	return n;
}

// The named arguments fill every register, so the last of them is on the stack
// ahead of the variable ones.
long after_nine(long a, long b, long c, long d, long e, long f, long g, long h, long i, ...) {
	va_list args;
	va_start(args, i);
	long v = va_arg(args, long);
	va_end(args);
	return a + b + c + d + e + f + g + h + i * 100 + v * 10000;
}

typedef long (*variadic_fn)(int, ...);
variadic_fn get_last_of(void) { return last_of; }

// Calls the function pointer that is the first of its variable arguments, the
// way that curl_easy_setopt is given a callback.
int call_variadic_cb(int v, ...) {
	va_list args;
	va_start(args, v);
	int (*fn)(int) = va_arg(args, int (*)(int));
	va_end(args);
	return fn(v);
}

// Tells a caller what it was passed as a bool and as a double.
int bool_and_double(bool b, double d) { return (b ? 1000 : 0) + (int)d; }

#include <stdlib.h>

// Takes ownership of s, as a '$' parameter allows.
size_t free_str(char *s) {
	size_t n = strlen(s);
	free(s);
	return n;
}

// Keeps s for good, as a '~' parameter allows.
static const char *kept_s;
void keep_str(const char *s) { kept_s = s; }
const char *kept_str(void) { return kept_s; }

// Fails, and keeps why for the thread that it failed on, as errno, GetLastError
// and glGetError do. The sleep gives a goroutine the time to change thread.
static __thread int thread_error;
int fail_with(int code) {
	thread_error = code;
	usleep(50);
	return -1;
}
int why_failed(void) { return thread_error; }

// Write to the string that they are given, as far as its NUL.
void upper_str(char *s) {
	for (; *s; s++) *s = (char)upper(*s);
}
void clear_str(char *s) { s[0] = 0; }

// Tells a caller whether it was passed NULL with room for something.
int null_with_room(void *p, size_t n) { return p == NULL && n > 0; }
int both_null(void *a, void *b) { return a == NULL && b == NULL; }

// Hands the function that it is given a function pointer of its own.
int call_with_twice(int (*outer)(int (*)(int))) { return outer(twice); }

// Calls a function with a float as its variable argument, which C passes as a double.
typedef double (*variadic_double_fn)(int, ...);
double call_variadic_float_cb(variadic_double_fn fn) {
	float f = 2.5f;
	return fn(1, f);
}

// Hands the function that it is given a long double, which no Go type is passed like.
int call_long_double_cb(int (*fn)(long double)) { return fn(1.5L); }
