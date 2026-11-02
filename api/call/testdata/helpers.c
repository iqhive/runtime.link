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

static int bumps;
void bump(void) { bumps++; }
int bumped(void) { return bumps; }

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
